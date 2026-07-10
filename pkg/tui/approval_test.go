package tui

import (
	"testing"
	"time"

	"github.com/helmetica-framework/cupel/pkg/revision"
)

func ptr(t time.Time) *time.Time { return &t }

func TestApprovalNoneWhenNil(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	state, text := approval(revision.Revision{ApprovedAt: nil}, now)
	if state != approvalNone {
		t.Errorf("state = %d, want approvalNone", state)
	}
	if text != "not approved" {
		t.Errorf("text = %q, want %q", text, "not approved")
	}
}

func TestApprovalApprovedWhenPastOrNow(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, 7, 8, 16, 1, 0, 0, time.UTC)
	state, text := approval(revision.Revision{ApprovedAt: ptr(at)}, now)
	if state != approvalApproved {
		t.Errorf("state = %d, want approvalApproved", state)
	}
	if text != "approved at: 2026-07-08 16:01" {
		t.Errorf("text = %q, want %q", text, "approved at: 2026-07-08 16:01")
	}

	// Exactly now counts as approved (boundary: not in the future).
	state, _ = approval(revision.Revision{ApprovedAt: ptr(now)}, now)
	if state != approvalApproved {
		t.Errorf("at==now: state = %d, want approvalApproved", state)
	}
}

func TestApprovalFutureWhenAfterNow(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, 7, 20, 16, 1, 0, 0, time.UTC)
	state, text := approval(revision.Revision{ApprovedAt: ptr(at)}, now)
	if state != approvalFuture {
		t.Errorf("state = %d, want approvalFuture", state)
	}
	if text != "approved at: 2026-07-20 16:01" {
		t.Errorf("text = %q, want %q", text, "approved at: 2026-07-20 16:01")
	}
}
