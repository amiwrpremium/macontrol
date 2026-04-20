package runner

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Fake is a deterministic Runner for unit tests. Register canned responses
// keyed by the full "name arg1 arg2 …" command line; Exec and Sudo both
// consult the same table.
type Fake struct {
	mu    sync.Mutex
	rules map[string]fakeResult
	calls []FakeCall
}

// FakeCall records one invocation.
type FakeCall struct {
	Sudo bool
	Name string
	Args []string
}

type fakeResult struct {
	stdout []byte
	err    error
}

// NewFake returns an empty Fake.
func NewFake() *Fake {
	return &Fake{rules: map[string]fakeResult{}}
}

// On registers a canned response. Match is "name arg1 arg2 …" with exact
// arg matching. Use OnPrefix for looser matching.
func (f *Fake) On(cmdline string, stdout string, err error) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules[cmdline] = fakeResult{stdout: []byte(stdout), err: err}
	return f
}

// Calls returns a snapshot of recorded invocations.
func (f *Fake) Calls() []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// Exec implements Runner.
func (f *Fake) Exec(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.dispatch(false, name, args)
}

// Sudo implements Runner.
func (f *Fake) Sudo(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.dispatch(true, name, args)
}

func (f *Fake) dispatch(sudo bool, name string, args []string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, FakeCall{Sudo: sudo, Name: name, Args: append([]string{}, args...)})

	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	if r, ok := f.rules[key]; ok {
		return r.stdout, r.err
	}
	// Try prefix match (useful when the caller passes a variable tail).
	for k, r := range f.rules {
		if strings.HasPrefix(key, k) {
			return r.stdout, r.err
		}
	}
	return nil, errors.New("runner.Fake: no rule for " + key)
}
