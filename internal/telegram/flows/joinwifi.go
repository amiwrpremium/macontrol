package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
)

// NewJoinWifi returns the two-step typed-input [Flow] that
// joins the Mac to a Wi-Fi network: ask for SSID, then ask
// for password, then call [wifi.Service.Join].
//
// Behavior:
//   - The flow is stateful across the two user replies; the
//     SSID is captured in [joinWifiFlow.ssid] before the
//     password prompt fires.
//   - Open networks: typing the literal "-" as the password is
//     treated as "no password" and Join is called with an
//     empty password string.
//   - Terminates after the join attempt — Done=true on both
//     success and Join failure.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleWifi] when the user taps "Join network…".
func NewJoinWifi(svc *wifi.Service) Flow {
	return &joinWifiFlow{svc: svc}
}

// joinWifiFlow is the [NewJoinWifi]-returned [Flow]. Holds
// the SSID across the two user replies and a phase flag so
// [Handle] knows which step to run.
//
// Field roles:
//   - svc is the [wifi.Service] used by the final Join call.
//   - ssid is the SSID captured in step 1; empty until the
//     user replies the first time.
//   - waitPass is the phase flag: false on the SSID step
//     (Handle's first call), true on the password step
//     (Handle's second call).
type joinWifiFlow struct {
	svc      *wifi.Service
	ssid     string
	waitPass bool
}

// Name returns the dispatcher log identifier "wif:join".
func (joinWifiFlow) Name() string { return "wif:join" }

// Start emits the SSID prompt. Called once when the flow is
// installed.
func (joinWifiFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the *SSID* to join. Reply `/cancel` to abort."}
}

// Handle advances the flow one step.
//
// Routing rules (first match wins):
//  1. waitPass == false (SSID step):
//     a. text trims to empty → "SSID can't be empty. Try
//     again." (NOT terminal — user re-prompted).
//     b. otherwise → store ssid, flip waitPass=true, prompt
//     for password (mention "-" for open networks).
//  2. waitPass == true (password step):
//     a. text == "-" → treat as empty password (open
//     network).
//     b. otherwise → use text verbatim as the password.
//     Then call [wifi.Service.Join]:
//     - err non-nil → "⚠ could not join `<ssid>`: `<err>`"
//     + Done.
//     - err nil + info.SSID empty → "✅ Joined — SSID
//     `(not associated — check password)` · iface
//     `<iface>`" + Done. The fallback covers the case
//     where the Mac accepted the network into the prefs
//     list but couldn't associate (typically wrong
//     password silently rejected by the AP).
//     - otherwise → "✅ Joined — SSID `<ssid>` · iface
//     `<iface>`" + Done.
func (f *joinWifiFlow) Handle(ctx context.Context, text string) Response {
	text = strings.TrimSpace(text)
	if !f.waitPass {
		if text == "" {
			return Response{Text: "SSID can't be empty. Try again."}
		}
		f.ssid = text
		f.waitPass = true
		return Response{Text: fmt.Sprintf("Now send the password for *%s*. Send `-` for an open network.", f.ssid)}
	}
	pwd := text
	if pwd == "-" {
		pwd = ""
	}
	info, err := f.svc.Join(ctx, f.ssid, pwd)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not join `%s`: `%v`", f.ssid, err), Done: true}
	}
	ssid := info.SSID
	if ssid == "" {
		ssid = "(not associated — check password)"
	}
	return Response{
		Text: fmt.Sprintf("✅ Joined — SSID `%s` · iface `%s`", ssid, info.Interface),
		Done: true,
	}
}
