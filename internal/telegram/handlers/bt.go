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
// bluetoothDispatch maps Bluetooth callback actions to handlers.
// "open"/"refresh" share a handler; "conn"/"disc" share one
// because they differ only in which service method to call.
var bluetoothDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":    handleBluetoothRefresh,
	"refresh": handleBluetoothRefresh,
	"toggle":  handleBluetoothToggle,
	"paired":  handleBluetoothPaired,
	"conn":    handleBluetoothConnOp,
	"disc":    handleBluetoothConnOp,
}

func handleBluetooth(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := bluetoothDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown bluetooth action.")
		return nil
	}
	return h(ctx, d, q, data)
}

func handleBluetoothRefresh(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Bluetooth.Get(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔵 *Bluetooth* — `blueutil` not installed?", err)
	}
	text, kb := keyboards.Bluetooth(st)
	return r.Edit(ctx, q, text, kb)
}

func handleBluetoothToggle(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Bluetooth.Toggle(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔵 *Bluetooth* — toggle failed", err)
	}
	text, kb := keyboards.Bluetooth(st)
	return r.Edit(ctx, q, text, kb)
}

func handleBluetoothPaired(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	devs, err := d.Services.Bluetooth.Paired(ctx)
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
}

func handleBluetoothConnOp(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Bluetooth
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
	return handleBluetoothPaired(ctx, d, q, callbacks.Data{Namespace: callbacks.NSBT, Action: "paired"})
}
