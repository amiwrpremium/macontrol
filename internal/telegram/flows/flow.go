package flows

import (
	"context"

	"github.com/go-telegram/bot/models"
)

// Response is the per-step output of a [Flow]. The dispatcher in
// the bot package consumes this to decide what to send back to
// the user and whether the flow is finished.
//
// Lifecycle:
//   - Constructed once per [Flow.Start] (the initial prompt) and
//     once per [Flow.Handle] (each user reply).
//   - Returned by value; flows never share a Response across
//     steps, so the dispatcher is free to mutate it before
//     forwarding (it doesn't, but the contract permits it).
//
// Field roles:
//
//   - Text is the message body to send back. An empty string means
//     "no reply this step" — useful when a flow wants to swallow
//     a message without acknowledging (rare).
//
//   - Markup attaches an optional reply keyboard (inline OR reply
//     OR ForceReply OR ReplyKeyboardRemove). Nil means "use the
//     default keyboard for this chat" — typically Telegram shows
//     no keyboard for normal text replies.
//
//   - Done signals the dispatcher to unwire this flow from the
//     chat's active-flow slot via [FlowManager.Finish]. Set on
//     terminal steps (success, /cancel, fatal error) so the next
//     plain-text message falls through to "no flow active".
//
//   - ParseMode picks the Telegram parse mode for Text. Empty
//     string means the dispatcher defaults to HTML (after running
//     Text through [bot.MDToHTML]) — most flows want this. Set
//     it explicitly only when the flow has already produced
//     final-form HTML or wants Telegram's MarkdownV2 (rare).
type Response struct {
	// Text is the message body to send back. Empty means no
	// reply for this step.
	Text string

	// Markup is the optional keyboard to attach to the reply.
	// Any of InlineKeyboardMarkup, ReplyKeyboardMarkup,
	// ReplyKeyboardRemove, or ForceReply.
	Markup models.ReplyMarkup

	// Done signals that the flow has reached a terminal state and
	// should be removed from the chat's active-flow slot.
	Done bool

	// ParseMode picks the Telegram parse mode for Text. Empty
	// string defers to the dispatcher's default (HTML via
	// [bot.MDToHTML]).
	ParseMode models.ParseMode
}

// Flow is the multi-step text-input contract. Anything in
// macontrol that asks the user a question and waits for typed
// reply implements Flow: clipboard set, Wi-Fi join,
// keep-awake duration, kill-PID, send notification, say (TTS),
// screen recording duration, brightness/volume "set exact value",
// shortcut name, timezone IANA name, and the search variants
// of the shortcut + timezone pickers.
//
// Lifecycle:
//   - Constructed by a per-flow `New…` constructor (e.g.
//     [NewClipSet], [NewJoinWifi]) wiring in the relevant domain
//     [Service]. The handler that opens the flow then installs
//     it on the chat via [Registry.Install].
//   - The dispatcher calls Start once (its return becomes the
//     opening prompt sent to the user) and Handle once per
//     subsequent text message until a Response with Done=true
//     unwires the flow.
//   - The "/cancel" command is consumed by the dispatcher BEFORE
//     reaching Handle — flows do not need to special-case it.
//
// Concurrency:
//   - Flows are not expected to be safe for concurrent calls.
//     The dispatcher serialises Start and Handle per chat (one
//     active flow per chat); a flow may freely mutate its own
//     state across calls.
type Flow interface {
	// Name returns a stable identifier for log lines, typically
	// "<namespace>:<action>", e.g. "tls:clip-set" or "wif:join".
	// Used by the dispatcher's "flow reply" log on send error.
	Name() string

	// Start yields the opening prompt sent to the user when the
	// flow is installed. Called exactly once per flow lifetime.
	// A Done=true Response from Start is unusual but valid —
	// signals the flow finished synchronously without needing a
	// user reply.
	Start(ctx context.Context) Response

	// Handle consumes the user's latest text message and produces
	// the next step. text is the raw message body — flows are
	// responsible for their own parsing, validation, and error
	// re-prompting.
	Handle(ctx context.Context, text string) Response
}

// Starter is the deferred-construction signature used by flow
// factories that need dependencies the [Registry] resolves at
// install time (rather than at handler-construction time). Most
// flows have a direct `New…` constructor and don't need this;
// Starter exists for future extension and is not currently used
// in production code paths.
//
// The Response returned alongside the Flow is used as the
// opening prompt — equivalent to what [Flow.Start] would have
// produced — so the registry can hand the user something
// immediately without a separate Start round-trip.
type Starter func(ctx context.Context) (Flow, Response)
