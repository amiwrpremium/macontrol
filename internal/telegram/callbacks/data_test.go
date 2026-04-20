package callbacks_test

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	s := callbacks.Encode("snd", "up", "5")
	if s != "snd:up:5" {
		t.Fatalf("encoded = %q", s)
	}
	d, err := callbacks.Decode(s)
	if err != nil {
		t.Fatal(err)
	}
	if d.Namespace != "snd" || d.Action != "up" || len(d.Args) != 1 || d.Args[0] != "5" {
		t.Fatalf("parsed = %+v", d)
	}
}

func TestDecode_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := callbacks.Decode(""); err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestDecode_RejectsShort(t *testing.T) {
	t.Parallel()
	if _, err := callbacks.Decode("snd"); err == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestDecode_RejectsOverflow(t *testing.T) {
	t.Parallel()
	raw := "snd:up:" + strings.Repeat("x", 100)
	if _, err := callbacks.Decode(raw); err == nil {
		t.Fatal("expected error for overlong callback data")
	}
}

func TestEncode_PanicsOnOverflow(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = callbacks.Encode("snd", "up", strings.Repeat("x", 100))
}
