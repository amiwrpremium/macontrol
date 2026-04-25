package music_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// getCmd is the joined command-line key registered with
// runner.Fake for the lightweight Get path. Mirrors the order
// in music.metadataFields.
const getCmd = "nowplaying-cli get title album artist duration elapsedTime playbackRate contentItemIdentifier"

// getWithArtworkCmd is the same plus the trailing artworkData
// field used by GetWithArtwork.
const getWithArtworkCmd = getCmd + " artworkData"

// fake registers a get rule with stdout canned and returns the
// fake. Helper used by every Get-side test below.
func fake(canned string) *runner.Fake {
	return runner.NewFake().On(getCmd, canned, nil)
}

func TestGet_AllFields(t *testing.T) {
	t.Parallel()
	stdout := "Mr. Brightside\nHot Fuss\nThe Killers\n222.5\n63.0\n1.0\nspotify:track:abc\n"
	np, err := music.New(fake(stdout)).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Title != "Mr. Brightside" || np.Album != "Hot Fuss" || np.Artist != "The Killers" {
		t.Fatalf("metadata mismatch: %+v", np)
	}
	if np.Duration != 222500*time.Millisecond {
		t.Fatalf("duration = %v", np.Duration)
	}
	if np.Elapsed != 63*time.Second {
		t.Fatalf("elapsed = %v", np.Elapsed)
	}
	if np.PlaybackRate != 1.0 {
		t.Fatalf("rate = %v", np.PlaybackRate)
	}
	if np.TrackID != "spotify:track:abc" {
		t.Fatalf("track id = %q", np.TrackID)
	}
	if np.Artwork != nil {
		t.Fatalf("Get must NOT populate Artwork; got %d bytes", len(np.Artwork))
	}
}

func TestGet_EmptyFields(t *testing.T) {
	t.Parallel()
	np, err := music.New(fake("")).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Title != "" || np.Album != "" || np.Artist != "" ||
		np.Duration != 0 || np.Elapsed != 0 || np.PlaybackRate != 0 ||
		np.TrackID != "" || np.Artwork != nil {
		t.Fatalf("expected zero value; got %+v", np)
	}
}

func TestGet_PartialFields(t *testing.T) {
	t.Parallel()
	// Only title + duration; the other lines are blank or absent.
	stdout := "Live Stream\n\n\n0\n0\n1\nhttp://stream/123\n"
	np, err := music.New(fake(stdout)).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Title != "Live Stream" || np.Album != "" || np.Artist != "" {
		t.Fatalf("metadata mismatch: %+v", np)
	}
	if np.Duration != 0 || np.Elapsed != 0 {
		t.Fatalf("expected zero times; got dur=%v elapsed=%v", np.Duration, np.Elapsed)
	}
}

func TestGet_UnicodeMetadata(t *testing.T) {
	t.Parallel()
	stdout := "Café del Mar\nVolumen 1\nDJ José\n300\n0\n1\nx\n"
	np, err := music.New(fake(stdout)).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Title != "Café del Mar" || np.Artist != "DJ José" {
		t.Fatalf("unicode lost: %+v", np)
	}
}

func TestGet_GarbageNumeric(t *testing.T) {
	t.Parallel()
	// Non-numeric duration / elapsed must NOT fail the whole snapshot.
	// They silently zero, which matches the lenient-parser contract.
	stdout := "T\nA\nR\nbananas\nfoo\nbar\nid\n"
	np, err := music.New(fake(stdout)).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Title != "T" {
		t.Fatalf("title = %q", np.Title)
	}
	if np.Duration != 0 || np.Elapsed != 0 || np.PlaybackRate != 0 {
		t.Fatalf("non-numeric should zero; got %+v", np)
	}
}

func TestGet_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(getCmd, "", errors.New("exec: nowplaying-cli not found"))
	if _, err := music.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected runner error to bubble")
	}
}

func TestIsPlaying(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rate float64
		want bool
	}{
		{1.0, true},
		{2.0, true},
		{0.0, false},
		{-1.0, false},
	}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			t.Parallel()
			np := music.NowPlaying{PlaybackRate: c.rate}
			if got := np.IsPlaying(); got != c.want {
				t.Fatalf("rate=%v got %v want %v", c.rate, got, c.want)
			}
		})
	}
}

func TestGetWithArtwork_DecodesBase64(t *testing.T) {
	t.Parallel()
	pixel := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a} // PNG header
	enc := base64.StdEncoding.EncodeToString(pixel)
	stdout := "T\nA\nR\n100\n50\n1\nid\n" + enc + "\n"
	f := runner.NewFake().On(getWithArtworkCmd, stdout, nil)
	np, err := music.New(f).GetWithArtwork(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(np.Artwork) != string(pixel) {
		t.Fatalf("artwork bytes mismatch: got %x want %x", np.Artwork, pixel)
	}
}

func TestGetWithArtwork_EmptyArtworkLine(t *testing.T) {
	t.Parallel()
	// Some players don't expose artwork; the trailing line is blank.
	stdout := "T\nA\nR\n100\n50\n1\nid\n\n"
	f := runner.NewFake().On(getWithArtworkCmd, stdout, nil)
	np, err := music.New(f).GetWithArtwork(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if np.Artwork != nil {
		t.Fatalf("expected nil artwork; got %d bytes", len(np.Artwork))
	}
	if np.Title != "T" {
		t.Fatalf("metadata corrupted on no-artwork path: %+v", np)
	}
}

func TestGetWithArtwork_BadBase64(t *testing.T) {
	t.Parallel()
	stdout := "T\nA\nR\n100\n50\n1\nid\nnot-base64!!!\n"
	f := runner.NewFake().On(getWithArtworkCmd, stdout, nil)
	if _, err := music.New(f).GetWithArtwork(context.Background()); err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestGetWithArtwork_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(getWithArtworkCmd, "", errors.New("boom"))
	if _, err := music.New(f).GetWithArtwork(context.Background()); err == nil {
		t.Fatal("expected runner error")
	}
}

func TestVerbs_IssueExpectedCommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func(*music.Service) error
		cmd  string
	}{
		{"Play", func(s *music.Service) error { return s.Play(context.Background()) }, "nowplaying-cli play"},
		{"Pause", func(s *music.Service) error { return s.Pause(context.Background()) }, "nowplaying-cli pause"},
		{"TogglePlayPause", func(s *music.Service) error { return s.TogglePlayPause(context.Background()) }, "nowplaying-cli togglePlayPause"},
		{"Next", func(s *music.Service) error { return s.Next(context.Background()) }, "nowplaying-cli next"},
		{"Previous", func(s *music.Service) error { return s.Previous(context.Background()) }, "nowplaying-cli previous"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().On(c.cmd, "", nil)
			if err := c.fn(music.New(f)); err != nil {
				t.Fatal(err)
			}
			calls := f.Calls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call; got %d", len(calls))
			}
			joined := calls[0].Name
			for _, a := range calls[0].Args {
				joined += " " + a
			}
			if joined != c.cmd {
				t.Fatalf("argv = %q; want %q", joined, c.cmd)
			}
		})
	}
}

func TestVerbs_PassthroughError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("nowplaying-cli play", "", errors.New("no media"))
	if err := music.New(f).Play(context.Background()); err == nil {
		t.Fatal("expected runner error to bubble")
	}
}

func TestSeek_PositiveSeconds(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("nowplaying-cli seek 60", "", nil)
	if err := music.New(f).Seek(context.Background(), 60); err != nil {
		t.Fatal(err)
	}
}

func TestSeek_NegativeClampsToZero(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("nowplaying-cli seek 0", "", nil)
	if err := music.New(f).Seek(context.Background(), -10); err != nil {
		t.Fatal(err)
	}
	calls := f.Calls()
	if len(calls) != 1 || calls[0].Args[1] != "0" {
		t.Fatalf("expected seek 0; got %+v", calls)
	}
}

func TestSeek_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("nowplaying-cli seek 30", "", errors.New("not seekable"))
	if err := music.New(f).Seek(context.Background(), 30); err == nil {
		t.Fatal("expected runner error")
	}
}
