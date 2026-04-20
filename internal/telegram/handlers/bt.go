package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleBluetooth(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Bluetooth

	switch data.Action {
	case "open", "refresh":
		r.Ack(ctx, q)
		st, err := svc.Get(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔵 *Bluetooth* — `blueutil` not installed?", err)
		}
		text, kb := keyboards.Bluetooth(st)
		return r.Edit(ctx, q, text, kb)

	case "toggle":
		r.Ack(ctx, q)
		st, err := svc.Toggle(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔵 *Bluetooth* — toggle failed", err)
		}
		text, kb := keyboards.Bluetooth(st)
		return r.Edit(ctx, q, text, kb)

	case "paired":
		r.Ack(ctx, q)
		devs, err := svc.Paired(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔵 *Bluetooth devices* — unavailable", err)
		}
		rows := make([]keyboards.BluetoothDeviceRow, 0, len(devs))
		for _, dev := range devs {
			id := d.ShortMap.Put(dev.Address)
			label := dev.Name
			if label == "" {
				label = dev.Address
			}
			rows = append(rows, keyboards.BluetoothDeviceRow{
				Label:     label,
				ShortID:   id,
				Connected: dev.Connected,
			})
		}
		text, kb := keyboards.BluetoothDevices(rows)
		return r.Edit(ctx, q, text, kb)

	case "conn", "disc":
		if len(data.Args) == 0 {
			r.Toast(ctx, q, "Missing device id.")
			return nil
		}
		addr, ok := d.ShortMap.Get(data.Args[0])
		if !ok {
			r.Toast(ctx, q, "Session expired; refresh the device list.")
			return nil
		}
		r.Toast(ctx, q, fmt.Sprintf("Talking to %s…", addr))
		var err error
		if data.Action == "conn" {
			err = svc.Connect(ctx, addr)
		} else {
			err = svc.Disconnect(ctx, addr)
		}
		if err != nil {
			return errEdit(ctx, r, q, "🔵 *Bluetooth* — device op failed", err)
		}
		// Re-render device list to show updated connected state.
		return handleBluetooth(ctx, d, q, callbacks.Data{Namespace: callbacks.NSBT, Action: "paired"})
	}
	r.Toast(ctx, q, "Unknown bluetooth action.")
	return nil
}
