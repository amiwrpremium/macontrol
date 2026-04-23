package tools

import (
	"context"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service exposes utility helpers.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// ClipboardRead returns current clipboard text.
func (s *Service) ClipboardRead(ctx context.Context) (string, error) {
	out, err := s.r.Exec(ctx, "pbpaste")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ClipboardWrite replaces clipboard text.
func (s *Service) ClipboardWrite(ctx context.Context, text string) error {
	// Use AppleScript to avoid piping to pbcopy through our runner (which
	// doesn't expose stdin). Escape double-quotes and backslashes so the
	// AppleScript string literal remains well-formed for arbitrary input.
	escaped := escapeForAppleScript(text)
	_, err := s.r.Exec(ctx, "osascript", "-e",
		`set the clipboard to "`+escaped+`"`)
	return err
}

func escapeForAppleScript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
