package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Bluetooth renders the 🔵 Bluetooth dashboard.
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
			Nav(),
		},
	}
	return
}

// BluetoothDeviceRow is one row in the device-picker keyboard. The shortID
// is placed in callback_data so long MAC strings never overflow the 64-byte
// limit.
type BluetoothDeviceRow struct {
	Label     string
	ShortID   string
	Connected bool
}

// BluetoothDevices builds the device-picker keyboard.
func BluetoothDevices(devs []BluetoothDeviceRow) (text string, markup *models.InlineKeyboardMarkup) {
	if len(devs) == 0 {
		text = "🔵 *Bluetooth Devices*\n\n_No paired devices._"
		markup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{Nav()}}
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
