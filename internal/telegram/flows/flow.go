package flows

import (
	"context"

	"github.com/go-telegram/bot/models"
)

// Response is what a Flow returns for each step.
type Response struct {
	// Text is the message to send back. Empty means no reply.
	Text string
	// Markup attaches an optional keyboard to the reply.
	Markup models.ReplyMarkup
	// Done signals the manager to clear this flow from state.
	Done bool
	// ParseMode for the reply (defaults to Markdown).
	ParseMode models.ParseMode
}

// Flow is invoked for each incoming text message in a chat where a flow is
// active. text is the user's latest message.
type Flow interface {
	// Name identifies the flow for logging; usually "<ns>:<action>".
	Name() string
	// Start yields the initial prompt when the flow is installed.
	Start(ctx context.Context) Response
	// Handle consumes the user's next message and produces the next step.
	Handle(ctx context.Context, text string) Response
}

// Starter is implemented by flow constructors that need deps from the
// registry's init step.
type Starter func(ctx context.Context) (Flow, Response)
