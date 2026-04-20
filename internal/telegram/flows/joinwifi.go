package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
)

// NewJoinWifi asks for an SSID, then a password, then joins.
func NewJoinWifi(svc *wifi.Service) Flow {
	return &joinWifiFlow{svc: svc}
}

type joinWifiFlow struct {
	svc      *wifi.Service
	ssid     string
	waitPass bool
}

func (joinWifiFlow) Name() string { return "wif:join" }

func (joinWifiFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the *SSID* to join. Reply `/cancel` to abort."}
}

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
