package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/media"
)

// NewRecord asks for a duration (seconds), records that long, and calls
// sendVideo with the resulting file path. sendVideo is responsible for
// uploading and removing the file.
func NewRecord(svc *media.Service, chatID int64, sendVideo func(ctx context.Context, path string) error) Flow {
	return &recordFlow{svc: svc, chatID: chatID, send: sendVideo}
}

type recordFlow struct {
	svc    *media.Service
	chatID int64
	send   func(ctx context.Context, path string) error
}

func (recordFlow) Name() string { return "med:record" }

func (recordFlow) Start(_ context.Context) Response {
	return Response{Text: "Record for how many seconds? (1-120). Reply `/cancel` to abort."}
}

func (f *recordFlow) Handle(ctx context.Context, text string) Response {
	secs, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || secs < 1 || secs > 120 {
		return Response{Text: "Please reply with an integer between 1 and 120."}
	}
	// Allow extra time beyond the recording window for upload.
	deadline, _ := ctx.Deadline()
	if deadline.Before(time.Now().Add(time.Duration(secs+30) * time.Second)) {
		c, cancel := context.WithTimeout(context.Background(), time.Duration(secs+30)*time.Second)
		defer cancel()
		ctx = c
	}
	path, err := f.svc.Record(ctx, time.Duration(secs)*time.Second)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ record failed: `%v`", err), Done: true}
	}
	if err := f.send(ctx, path); err != nil {
		return Response{Text: fmt.Sprintf("⚠ upload failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Recorded %ds.", secs), Done: true}
}
