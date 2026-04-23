// Package keyboards builds Telegram reply and inline keyboards.
//
// Pure UI: no side effects, no domain imports beyond shared structs. The
// separation keeps layout unit-testable and lets the handler layer decide
// which keyboard to send without embedding Telegram-specific rendering in
// the domain code. Every exported builder returns a fully-formed
// [*github.com/go-telegram/bot/models.InlineKeyboardMarkup]; callback
// payloads come from [internal/telegram/callbacks] to keep
// namespace-and-action encoding centralised.
//
// The package also hosts a few small formatting helpers
// ([TruncateShortcutLabel], [FlagFromISO2]) that belong with the layout
// because they shape user-visible strings.
package keyboards
