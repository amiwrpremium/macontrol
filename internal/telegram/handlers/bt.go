package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleBluetooth is the Bluetooth dashboard's callback
// dispatcher. Reached via the [callbacks.NSBT] namespace from
// any tap on the 🔵 Bluetooth menu, the paired-devices list,
// or a per-device connect/disconnect button.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → run [bluetooth.Service.Get], render
//     the dashboard via [keyboards.Bluetooth]. Get failures
//     surface with the "blueutil not installed?" hint because
//     the most likely cause is the missing brew formula.
//  2. "toggle"           → run [bluetooth.Service.Toggle]
//     (which composes Get + SetPower); re-render the
//     dashboard with the post-toggle state.
//  3. "paired"           → list paired devices via
//     [bluetooth.Service.Paired], park each MAC in the
//     [bot.Deps.ShortMap] (MAC strings are short but using
//     the ShortMap keeps the callback shape uniform with
//     other lists), build [keyboards.BluetoothDeviceRow]
//     rows, render the device list keyboard.
//  4. "conn" / "disc"    → connect/disconnect a device by
//     ShortMap-resolved MAC. After the op (success or
//     failure on the connect path), recursively dispatches
//     the "paired" action to re-render the list with the
//     post-op connected-state. This is the one place in the
//     codebase where a handler calls itself.
//
// "Session expired; refresh the device list." is the
// user-visible message when the ShortMap lookup misses
// (15-min TTL — the user kept a stale dashboard open).
//
// Unknown actions fall through to a "Unknown bluetooth action."
// toast.
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
