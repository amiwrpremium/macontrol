package media_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/media"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// helper to find the captured screencapture/imagesnap argv from a Fake.
func lastArgs(f *runner.Fake, name string) []string {
	for _, c := range f.Calls() {
		if c.Name == name {
			return c.Args
		}
	}
	return nil
}

// ---------------- Screenshot ----------------

func TestScreenshot_DefaultOpts(t *testing.T) {
	t.Parallel()
	// Match any screencapture invocation by prefix.
	f := runner.NewFake().On("screencapture ", "", nil)
	svc := media.New(f)
	path, err := svc.Screenshot(context.Background(), media.ScreenshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	args := lastArgs(f, "screencapture")
	// Default: no -x, no -T, no -D; only the path argument.
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %v", args)
	}
	if !strings.HasSuffix(args[0], ".png") {
		t.Errorf("expected .png path, got %q", args[0])
	}
}

func TestScreenshot_AllOpts(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", nil)
	path, err := media.New(f).Screenshot(context.Background(), media.ScreenshotOpts{
		Silent: true, Delay: 3, Display: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	args := lastArgs(f, "screencapture")
	// Expected order: -x  -T 3  -D 2  <path>
	joined := strings.Join(args, " ")
	for _, flag := range []string{"-x", "-T 3", "-D 2"} {
		if !strings.Contains(joined, flag) {
			t.Errorf("expected flag %q in %q", flag, joined)
		}
	}
}

func TestScreenshot_FailureRemovesTempfile(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", errors.New("denied"))
	path, err := media.New(f).Screenshot(context.Background(), media.ScreenshotOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if path != "" {
		t.Errorf("expected empty path on failure, got %q", path)
	}
}

func TestScreenshot_UniqueTempPaths(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", nil)
	svc := media.New(f)
	paths := make(map[string]bool)
	for i := 0; i < 5; i++ {
		p, err := svc.Screenshot(context.Background(), media.ScreenshotOpts{})
		if err != nil {
			t.Fatal(err)
		}
		paths[p] = true
		os.Remove(p)
	}
	if len(paths) != 5 {
		t.Fatalf("expected 5 unique paths, got %d", len(paths))
	}
}

// ---------------- Record ----------------

func TestRecord_RejectsNonPositive(t *testing.T) {
	t.Parallel()
	svc := media.New(runner.NewFake())
	for _, d := range []time.Duration{0, -1 * time.Second} {
		if _, err := svc.Record(context.Background(), d); err == nil {
			t.Errorf("duration %s should be rejected", d)
		}
	}
}

func TestRecord_FlagsIncludeVideoDuration(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", nil)
	path, err := media.New(f).Record(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	args := lastArgs(f, "screencapture")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v") {
		t.Errorf("expected -v video flag")
	}
	if !strings.Contains(joined, "-V 10") {
		t.Errorf("expected -V 10, got %q", joined)
	}
	if !strings.HasSuffix(path, ".mov") {
		t.Errorf("expected .mov suffix, got %q", path)
	}
}

func TestRecord_FailureRemovesTempfile(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", errors.New("TCC denied"))
	path, err := media.New(f).Record(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

// ---------------- Photo ----------------

func TestPhoto_InvokesImagesnap(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("imagesnap ", "", nil)
	path, err := media.New(f).Photo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	args := lastArgs(f, "imagesnap")
	// Expected: -q -w 1 <path>
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-q") || !strings.Contains(joined, "-w 1") {
		t.Errorf("expected -q -w 1 in args, got %q", joined)
	}
	if filepath.Ext(path) != ".jpg" {
		t.Errorf("expected .jpg, got %q", path)
	}
}

func TestPhoto_FailureCleans(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("imagesnap ", "", errors.New("no camera"))
	path, err := media.New(f).Photo(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}
