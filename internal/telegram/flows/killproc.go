package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
)

// NewKillProc returns the typed-input [Flow] that sends
// SIGTERM to a process identified by either a PID or a
// process name.
//
// Behavior:
//   - The flow autodetects PID vs. name: if the trimmed reply
//     parses as an integer, [system.Service.Kill] is called
//     with that PID; otherwise [system.Service.KillByName]
//     runs `killall <name>`.
//   - One-shot: terminates after the first kill attempt
//     (Done=true on success and on failure).
//   - SIGTERM only — there's no escalation to SIGKILL through
//     this flow. The Top-10 → per-process drill-down provides
//     SIGKILL via [keyboards.SystemKillConfirm].
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleSystem] when the user taps "Kill process…".
func NewKillProc(svc *system.Service) Flow {
	return &killProcFlow{svc: svc}
}

// killProcFlow is the [NewKillProc]-returned [Flow]. Holds
// only the [system.Service] reference; one-shot.
type killProcFlow struct{ svc *system.Service }

// Name returns the dispatcher log identifier "sys:kill".
func (killProcFlow) Name() string { return "sys:kill" }

// Start emits the typed-input prompt that accepts either a
// PID or a process name.
func (killProcFlow) Start(_ context.Context) Response {
	return Response{Text: "Send a PID (integer) or a process name to kill. Reply `/cancel` to abort."}
}

// Handle parses the user's reply and dispatches to either
// [system.Service.Kill] (PID branch) or
// [system.Service.KillByName] (name branch).
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty. Send a PID or name." (NOT
//     terminal — user re-prompted).
//  2. text parses as int (strconv.Atoi succeeds) → PID branch:
//     a. Kill returns non-nil err → "⚠ kill <pid> failed:
//     `<err>`" + Done.
//     b. Otherwise → "✅ SIGTERM sent to pid `<pid>`." +
//     Done.
//  3. text doesn't parse as int → name branch via killall:
//     a. KillByName returns non-nil err → "⚠ killall <name>
//     failed: `<err>`" + Done.
//     b. Otherwise → "✅ `killall <name>` done." + Done.
//
// The PID-branch parse is greedy: "1234abc" fails as int and
// falls through to killall, which then fails because killall
// rejects names with non-name characters. The two-tier error
// gives the user a useful diagnostic either way.
func (f *killProcFlow) Handle(ctx context.Context, text string) Response {
	text = strings.TrimSpace(text)
	if text == "" {
		return Response{Text: "Empty. Send a PID or name."}
	}
	if pid, err := strconv.Atoi(text); err == nil {
		if err := f.svc.Kill(ctx, pid); err != nil {
			return Response{Text: fmt.Sprintf("⚠ kill %d failed: `%v`", pid, err), Done: true}
		}
		return Response{Text: fmt.Sprintf("✅ SIGTERM sent to pid `%d`.", pid), Done: true}
	}
	if err := f.svc.KillByName(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ killall %s failed: `%v`", text, err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ `killall %s` done.", text), Done: true}
}
