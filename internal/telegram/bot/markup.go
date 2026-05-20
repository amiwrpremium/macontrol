package bot

import (
	"html"
	"strings"
)

// MDToHTML translates the legacy Markdown-style markers used
// throughout the codebase into Telegram's HTML parse-mode
// equivalents.
//
// Mapping (in order applied):
//   - HTML escape first — every input goes through
//     [html.EscapeString] so dynamic content (error strings,
//     hostnames, SSIDs, untrusted user input) can't inject
//     `<` / `>` / `&` and break the parse mode.
//   - ``` … ``` (fenced) → <pre> … </pre>. Done FIRST so
//     fenced content isn't re-processed by the inline
//     converters.
//   - ` … ` → <code> … </code>.
//   - * … * → <b> … </b>.
//   - _ … _ → <i> … </i>.
//
// Why this lives in the bot package: both the handler [Reply]
// helpers and the dispatcher's flow-reply path in
// [Deps.dispatchFlow] need it. Handlers can't be the home
// (the dispatcher would import handlers, breaking layering);
// a separate util package felt like over-architecture. Bot
// owns it.
//
// Returns the escaped + tag-converted string ready to send
// with [models.ParseModeHTML].
func MDToHTML(s string) string {
	s = html.EscapeString(s)
	// Fenced code blocks first so their inner text isn't reprocessed.
	s = alternate(s, "```", "<pre>", "</pre>")
	s = alternate(s, "`", "<code>", "</code>")
	s = alternate(s, "*", "<b>", "</b>")
	s = alternate(s, "_", "<i>", "</i>")
	return s
}

// alternate replaces every paired occurrence of delim in s
// with the open / closeT tag pair, leaving odd / unpaired
// trailing delims as literals.
//
// Behavior:
//  1. Split s on delim. With N occurrences of delim, splits
//     into N+1 parts.
//  2. Compute `usable = (N/2)*2` — the count of delims that
//     have a partner. Any orphan trailing delim (when N is
//     odd) is preserved as a literal so malformed user input
//     still produces readable output.
//  3. Emit parts[0], then alternate open/closeT/open/closeT…
//     until usable runs out. From there, emit any remaining
//     delims as literals.
//
// Example: alternate("a*b*c*d", "*", "<b>", "</b>") →
// "a<b>b</b>c*d" (third '*' is orphaned).
//
// Doesn't recurse — fenced blocks must be processed BEFORE
// inline `code` to avoid the inline converter eating the
// fence's content.
func alternate(s, delim, open, closeT string) string {
	parts := strings.Split(s, delim)
	n := len(parts) - 1 // number of delim occurrences
	if n == 0 {
		return s
	}
	usable := (n / 2) * 2 // delims converted to tags; orphan (if any) stays literal
	var b strings.Builder
	b.Grow(len(s))
	for i, p := range parts {
		b.WriteString(p)
		if i == len(parts)-1 {
			break
		}
		switch {
		case i >= usable:
			b.WriteString(delim) // orphan — preserve as literal
		case i%2 == 0:
			b.WriteString(open)
		default:
			b.WriteString(closeT)
		}
	}
	return b.String()
}
