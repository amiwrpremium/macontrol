// Package flows implements multi-step Telegram conversations.
//
// A [Flow] is a stateful callback associated with a chat: each incoming
// message for that chat is fed to [Flow.Handle], which returns a
// [Response] describing the reply and whether the flow is done. The
// [Registry] owns the chat-to-flow map with a configurable inactivity
// TTL, evicts stalled flows in a background janitor, and provides
// cancellation hooks for the `/cancel` command.
//
// Flow constructors live next to their owning domain (setvolume,
// setbrightness, joinwifi, killproc, keepawake, record, notify, say,
// clipset, shortcut, timezone, plus the "search" variants that filter
// large lists by substring). A flow never speaks to Telegram directly:
// it returns a [Response] and the handler layer owns the transport.
package flows
