package bot

import (
	"html"
	"strings"
)

// MDToHTML translates legacy Markdown-style markers (*bold*, _italic_,
// `code`, ```fence```) into Telegram's HTML parse-mode equivalents
// (<b>, <i>, <code>, <pre>). HTML meta characters in the input are
// escaped first so dynamic content (error strings, hostnames, SSIDs)
// can't inject markup.
//
// It lives in the bot package rather than handlers so both the handler
// Reply helpers and the bot-level flow-reply path can call it without
// an import cycle.
func MDToHTML(s string) string {
	s = html.EscapeString(s)
	// Fenced code blocks first so their inner text isn't reprocessed.
	s = alternate(s, "```", "<pre>", "</pre>")
	s = alternate(s, "`", "<code>", "</code>")
	s = alternate(s, "*", "<b>", "</b>")
	s = alternate(s, "_", "<i>", "</i>")
	return s
}

// alternate replaces every pair of delim markers in s with open then
// close. An odd (unbalanced) trailing delim is emitted as a literal so
// callers with malformed markup still produce readable output.
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
