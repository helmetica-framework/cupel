package tui

import (
	"time"

	"github.com/helmetica-framework/cupel/pkg/revision"
)

// approvalState classifies a revision's approval status for the ledger view.
type approvalState int

const (
	approvalNone     approvalState = iota // no ApprovedAt: unapproved
	approvalApproved                      // ApprovedAt set, at or before now
	approvalFuture                        // ApprovedAt set, after now
)

// approval reports rev's approval state and the status-line text to render.
// now is the model's clock so callers stay deterministic in tests.
func approval(rev revision.Revision, now time.Time) (approvalState, string) {
	if rev.ApprovedAt == nil {
		return approvalNone, "not approved"
	}

	approvedText := "approved at: " + rev.ApprovedAt.Format("2006-01-02 15:04")

	if rev.ApprovedAt.After(now) {
		return approvalFuture, approvedText
	}

	return approvalApproved, approvedText
}
