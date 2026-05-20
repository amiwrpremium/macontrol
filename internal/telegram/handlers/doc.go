// Package handlers implements the bot's command and callback routers.
//
// A handler is a thin layer between Telegram and the domain services:
// parse input, call the service, build a [internal/telegram/keyboards]
// layout, and edit or send the resulting message. [CommandRouter] owns
// the `/…` slash-command dispatch, [CallbackRouter] owns inline-keyboard
// button callbacks keyed by the namespaces in
// [internal/telegram/callbacks].
//
// The [Reply] helper centralises the Send/Edit/Toast pattern so handlers
// don't duplicate the Markdown-to-HTML conversion or the typed-nil
// reply_markup workaround. Per-domain handler files (wif.go, sys.go,
// snd.go, dsp.go, bt.go, med.go, pwr.go, ntf.go, tls.go, bat.go,
// nav.go, shim.go) each own one callback namespace. [BootPing] and
// [ClearLegacyReplyKB] are exposed because the daemon boot-ping path in
// cmd/macontrol needs them.
package handlers
