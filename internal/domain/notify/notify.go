// Package notify sends desktop notifications and text-to-speech. Tries
// `terminal-notifier` first (richer UX), falls back to `osascript display
// notification` if the brew formula is missing.
package notify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Opts tunes a notification.
type Opts struct {
	Title string
	Body  string
	// Sound is the macOS sound name to play (e.g. "default", "Glass", "Ping").
	// Empty means silent.
	Sound string
}

// Service sends notifications.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Notify sends a desktop notification. If terminal-notifier is installed it
// is used; otherwise falls back to osascript. Returns which transport was
// used so the caller can surface that in the UI when useful.
func (s *Service) Notify(ctx context.Context, o Opts) (transport string, err error) {
	if o.Title == "" && o.Body == "" {
		return "", errors.New("notify: title or body required")
	}
	if s.hasTerminalNotifier() {
		return "terminal-notifier", s.viaTerminalNotifier(ctx, o)
	}
	return "osascript", s.viaOsascript(ctx, o)
}

// Say speaks text through the default TTS voice.
func (s *Service) Say(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("say: text is empty")
	}
	_, err := s.r.Exec(ctx, "say", text)
	return err
}

func (s *Service) hasTerminalNotifier() bool {
	_, err := exec.LookPath("terminal-notifier")
	return err == nil
}

func (s *Service) viaTerminalNotifier(ctx context.Context, o Opts) error {
	args := []string{"-group", "macontrol"}
	if o.Title != "" {
		args = append(args, "-title", o.Title)
	}
	args = append(args, "-message", o.Body)
	if o.Sound != "" {
		args = append(args, "-sound", o.Sound)
	}
	_, err := s.r.Exec(ctx, "terminal-notifier", args...)
	return err
}

func (s *Service) viaOsascript(ctx context.Context, o Opts) error {
	var b strings.Builder
	fmt.Fprintf(&b, `display notification %q`, o.Body)
	if o.Title != "" {
		fmt.Fprintf(&b, ` with title %q`, o.Title)
	}
	if o.Sound != "" {
		fmt.Fprintf(&b, ` sound name %q`, o.Sound)
	}
	_, err := s.r.Exec(ctx, "osascript", "-e", b.String())
	return err
}
