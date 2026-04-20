package bot

import "github.com/go-telegram/bot/models"

// Whitelist returns a predicate that accepts only updates from the allowed
// user IDs. Applied in the dispatch layer before any work is done.
type Whitelist map[int64]struct{}

// NewWhitelist builds a whitelist from a slice of IDs.
func NewWhitelist(ids []int64) Whitelist {
	w := Whitelist{}
	for _, id := range ids {
		w[id] = struct{}{}
	}
	return w
}

// Allows reports whether the update's sender is on the whitelist.
func (w Whitelist) Allows(update *models.Update) bool {
	id := senderID(update)
	if id == 0 {
		return false
	}
	_, ok := w[id]
	return ok
}

// Members returns the raw id list (stable order not guaranteed).
func (w Whitelist) Members() []int64 {
	out := make([]int64, 0, len(w))
	for id := range w {
		out = append(out, id)
	}
	return out
}

func senderID(u *models.Update) int64 {
	switch {
	case u.CallbackQuery != nil:
		return u.CallbackQuery.From.ID
	case u.Message != nil && u.Message.From != nil:
		return u.Message.From.ID
	}
	return 0
}
