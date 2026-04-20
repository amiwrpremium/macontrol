package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestIsCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"/start", true},
		{"/menu something", true},
		{"/foo@bot", true},
		{"/", false},
		{"", false},
		{"hello", false},
		{"-flag", false},
	}
	for _, c := range cases {
		if got := isCommand(c.in); got != c.want {
			t.Errorf("isCommand(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSenderID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		u    *models.Update
		want int64
	}{
		{"message", &models.Update{Message: &models.Message{From: &models.User{ID: 123}}}, 123},
		{"callback", &models.Update{CallbackQuery: &models.CallbackQuery{From: models.User{ID: 456}}}, 456},
		{"message-no-from", &models.Update{Message: &models.Message{}}, 0},
		{"empty", &models.Update{}, 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := senderID(c.u); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}
