package runner

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Fake is a deterministic in-memory [Runner] for unit tests. Tests
// pre-register rules with [Fake.On] keyed by the rendered command
// line ("name arg1 arg2 …") and Fake echoes back the canned stdout
// or error.
//
// Lifecycle:
//   - Constructed once per test via [NewFake].
//   - Mutated through Fake.On as tests build their fixture, then
//     handed to the domain service under test.
//   - Inspected after the test via [Fake.Calls] to assert which
//     commands actually ran.
//
// Concurrency:
//   - All fields are guarded by mu. Safe for the production code
//     under test to call from multiple goroutines while the test
//     also calls Calls/On, which is uncommon but valid.
//
// Field roles:
//   - rules is the dispatch table. Keys are the trimmed rendered
//     command line; values are the canned stdout + err to return
//     on a match.
//   - calls is the recorder, appended on every invocation. Tests
//     read it via [Fake.Calls] to assert side-effects.
type Fake struct {
	// mu guards both rules and calls.
	mu sync.Mutex

	// rules maps "name arg1 arg2 …" → canned (stdout, err). Filled
	// by [Fake.On]. Matched both exactly and by prefix in
	// [Fake.dispatch].
	rules map[string]fakeResult

	// calls records every invocation in order, set by
	// [Fake.dispatch] before the rule lookup. Args is deep-copied
	// so later mutations by the caller do not affect the record.
	calls []FakeCall
}

// FakeCall is one recorded invocation through the [Fake] runner.
//
// Field roles:
//   - Sudo is true when the call entered through [Fake.Sudo],
//     false for both [Fake.Exec] and [Fake.ExecCombined]. Tests
//     can use it to assert "this code path went through sudo".
//     Note: ExecCombined and Exec are indistinguishable from this
//     record alone — see the smells list.
//   - Name and Args are the bare command name and its argument
//     slice, both as the production code passed them.
type FakeCall struct {
	// Sudo is true when the caller went through [Fake.Sudo],
	// false for [Fake.Exec] and [Fake.ExecCombined].
	Sudo bool

	// Name is the bare command name that was invoked.
	Name string

	// Args is a deep-copy of the argument slice the caller
	// supplied; safe to retain across the test even if the caller
	// later mutates its slice.
	Args []string
}

// fakeResult is the canned response stored per rule. Internal so
// tests register via [Fake.On] rather than constructing the struct
// directly.
type fakeResult struct {
	// stdout is returned as the success payload (or as the
	// partial-stdout slice alongside err on failure).
	stdout []byte

	// err, when non-nil, is returned as the second value from the
	// dispatched runner method. nil means the rule represents a
	// successful invocation.
	err error
}

// NewFake returns an empty [Fake] with an initialised rules map.
// The result satisfies [Runner].
func NewFake() *Fake {
	return &Fake{rules: map[string]fakeResult{}}
}

// On registers a canned response keyed by the rendered command
// line ("name arg1 arg2 …", joined with single spaces and trimmed).
// Returns the receiver so test fixtures can chain rule
// registrations.
//
// Behavior:
//   - Overwrites any existing rule with the same key.
//   - The key is matched both exactly and as a prefix in
//     [Fake.dispatch] — see the smells list for the prefix-match
//     pitfall.
//   - The same rules table services Exec, Sudo, and ExecCombined;
//     tests cannot register different responses per method.
func (f *Fake) On(cmdline string, stdout string, err error) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules[cmdline] = fakeResult{stdout: []byte(stdout), err: err}
	return f
}

// Calls returns a snapshot of every invocation recorded so far,
// in chronological order. The returned slice is a copy; subsequent
// invocations do not mutate it.
func (f *Fake) Calls() []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// Exec satisfies [Runner.Exec]. Records the call with Sudo=false
// then dispatches against the shared rules table.
func (f *Fake) Exec(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.dispatch(false, name, args)
}

// Sudo satisfies [Runner.Sudo]. Records the call with Sudo=true
// then dispatches against the shared rules table. The "-n" prefix
// added by the production [Exec.Sudo] is NOT prepended in the
// fake, so test rules should be keyed by the user-visible
// command, e.g. On("pmset -g batt", …).
func (f *Fake) Sudo(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.dispatch(true, name, args)
}

// ExecCombined satisfies [Runner.ExecCombined]. Records the call
// with Sudo=false (indistinguishable from Exec in the recorder)
// and dispatches against the same rules table. Tests register the
// combined-stream output via On("name args", "<merged>", err);
// the fake does not actually merge stdout and stderr — what the
// test puts in the rule is what the caller gets.
func (f *Fake) ExecCombined(_ context.Context, name string, args ...string) ([]byte, error) {
	return f.dispatch(false, name, args)
}

// dispatch is the shared lookup behind Exec, Sudo, and
// ExecCombined. Records the call first (so tests still observe
// invocations even when no rule matches), renders the lookup key,
// then tries an exact match followed by a prefix match.
//
// Routing rules (first match wins):
//  1. Exact key in rules ("name arg1 arg2 …").
//  2. Any registered key that is a prefix of the lookup key.
//  3. Otherwise: returns nil stdout and an error of the form
//     "runner.Fake: no rule for <key>".
//
// The prefix branch lets tests register a rule like
// "networksetup -setairportpower" and have it cover any interface
// suffix, but it also means a more-specific rule registered later
// can be shadowed by an earlier shorter one — see the smells list.
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
