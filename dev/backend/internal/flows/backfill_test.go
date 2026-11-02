package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBackfillHistoricalWindowAndBoundedPreview(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	dates, truncated, e := historicalOccurrences("0 */6 * * *", "UTC", start, end, end.Add(time.Hour))
	if e != nil || truncated || len(dates) != 5 || !dates[0].Equal(start) || !dates[4].Equal(end) {
		t.Fatalf("historical preview %v %v %v", dates, truncated, e)
	}
	dates, truncated, e = historicalOccurrences("* * * * *", "UTC", start, end, end.Add(time.Hour))
	if e != nil || !truncated || len(dates) != 200 {
		t.Fatal("history limit silently truncated", e)
	}
	if _, _, e = historicalOccurrences("* * * * *", "UTC", start, end, start); e == nil {
		t.Fatal("future scheduling accepted")
	}
	if _, _, e = historicalOccurrences("invalid", "UTC", start, end, end); e == nil {
		t.Fatal("invalid cron accepted")
	}
}
func TestBackfillApprovalContextCannotBeReusedAcrossOccurrences(t *testing.T) {
	run := RunRequest{ApprovalAction: "bios", ApprovalID: "old", ApprovalContext: operationApprovalContext("job", 0)}
	first := ApprovalDigest(run)
	run.ApprovalContext = operationApprovalContext("job", 1)
	if first == ApprovalDigest(run) {
		t.Fatal("approval was not bound to the occurrence")
	}
	run.ApprovalContext = replayContext("execution", "nonce")
	if first == ApprovalDigest(run) {
		t.Fatal("backfill approval reused for replay")
	}
}
func TestReservedOperationContextCannotUseGenericExecute(t *testing.T) {
	_, e := (&Service{}).Start(context.Background(), Scope{}, RunRequest{ApprovalContext: "backfill:job:0"}, "synthetic")
	var domain *Error
	if !errors.As(e, &domain) || domain.Code != "RESERVED_APPROVAL_CONTEXT_USE_OPERATION_ENDPOINT" {
		t.Fatalf("generic execute accepted reserved operation context: %v", e)
	}
}
