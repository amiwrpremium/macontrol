package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Bluetooth renders the 🔵 Bluetooth dashboard for the
// supplied [bluetooth.State] snapshot.
//
// Header rendering:
//   - "🔵 *Bluetooth* — `<on|off>`" — bare power indicator;
//     the device list lives behind the "Paired devices" drill-
//     down rather than being inlined.
//
// Keyboard rendering (3 rows):
//  1. Dynamic toggle + Paired devices. The toggle button text
//     flips between "⏻ Turn on" (when off) and "⏻ Turn off"
//     (when on); both share the same `bt:toggle` callback —
//     [handlers.handleBluetooth] decides the direction from
//     the current state, not from the callback.
//  2. Refresh on its own row.
//  3. Standard Back/Home nav row from [NavWithBack].
//
// The single-callback toggle pattern (vs. separate `bt:on` /
// `bt:off`) means stale taps from a phone that hasn't seen the
// latest state still do the right thing — the handler reads
// the current power before flipping.
func Bluetooth(st bluetooth.State) (text string, markup *models.InlineKeyboardMarkup) {
	power := "off"
	if st.PowerOn {
		power = "on"
	}
	text = "🔵 *Bluetooth* — `" + power + "`"

	toggle := "⏻ Turn on"
	if st.PowerOn {
		toggle = "⏻ Turn off"
	}
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: toggle, CallbackData: callbacks.Encode(callbacks.NSBT, "toggle")},
				{Text: "📋 Paired devices", CallbackData: callbacks.Encode(callbacks.NSBT, "paired")},
			},
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSBT, "refresh")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}

// BluetoothDeviceRow is one row in the [BluetoothDevices]
// device-picker keyboard. Built by [handlers.handleBluetooth]
// from the [bluetooth.Device] slice plus a per-MAC ShortMap
// lookup.
//
// Field roles:
//   - Label is the user-visible device name (e.g. "AirPods Pro
//     2"). Falls back to the MAC address when the OS reports
//     an empty name.
//   - ShortID is the [callbacks.ShortMap] token that
//     [handlers.handleBluetooth] resolves back to a real MAC
//     address. Required because raw MACs are 17 bytes and the
//     namespace+action prefix would push the connect/disconnect
//     callback over the 64-byte Telegram limit on long names.
//   - Connected is the current connection state used by
//     [BluetoothDevices] to pick the action verb (conn vs.
//     disc) and the row icon (🔗 vs. ✂).
type BluetoothDeviceRow struct {
	Label     string
	ShortID   string
	Connected bool
}

// BluetoothDevices renders the paired-devices picker keyboard.
//
// Header rendering (first match wins):
//  1. Empty devs slice → "🔵 *Bluetooth Devices*\n\n_No
//     paired devices._" with a [NavWithBack] row pointing back
//     to the Bluetooth dashboard (`bt:open`). No per-device
//     rows.
//  2. Otherwise → "🔵 *Bluetooth Devices*\n\nTap a device to
//     toggle connection." plus one row per device.
//
// Keyboard rendering (per-device rows, then Back, then Home):
//   - Each device row is a single full-width button:
//     "🔗 <Label>" when disconnected (callback `bt:conn:<id>`),
//     "✂ <Label>" when connected (callback `bt:disc:<id>`).
//     The action verb is decided here, not by the handler, so
//     the per-row callback unambiguously names the intent.
//   - Trailing rows: "← Back" (→ `bt:open`) followed by the
//     standard [Nav] home row. Drill-back-to-parent over jump-
//     to-home — preserves the user's place in the device list.
func BluetoothDevices(devs []BluetoothDeviceRow) (text string, markup *models.InlineKeyboardMarkup) {
	if len(devs) == 0 {
		text = "🔵 *Bluetooth Devices*\n\n_No paired devices._"
		markup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
			NavWithBack(callbacks.NSBT, "open"),
		}}
		return
	}
	text = "🔵 *Bluetooth Devices*\n\nTap a device to toggle connection."
	rows := make([][]models.InlineKeyboardButton, 0, len(devs)+1)
	for _, d := range devs {
		action := "conn"
		prefix := "🔗"
		if d.Connected {
			action = "disc"
			prefix = "✂"
		}
		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text:         prefix + " " + d.Label,
				CallbackData: callbacks.Encode(callbacks.NSBT, action, d.ShortID),
			},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSBT, "open")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}
