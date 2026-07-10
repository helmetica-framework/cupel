package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/revision"
)

// fakePuller returns the demo chart for any ref, so RenderWith runs for real
// without touching the network.
type fakePuller struct{ ch *chart.Chart }

func (f fakePuller) Pull(string) (*chart.Chart, error) { return f.ch, nil }

var _ oci.Puller = fakePuller{}

func demoPuller(t *testing.T) fakePuller {
	t.Helper()
	raw, err := loader.Load("../render/testdata/demo")
	if err != nil {
		t.Fatalf("load demo: %v", err)
	}
	ch, ok := raw.(*chart.Chart)
	if !ok {
		t.Fatal("not a *chart.Chart")
	}
	return fakePuller{ch: ch}
}

func testFixtures(t *testing.T) (revision.Claim, []revision.Revision, oci.Puller, diff.Engine) {
	t.Helper()
	eng, err := diff.Get("linewise")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	claim := revision.Claim{
		OCI:     "oci://demo",
		Version: "1.0.0",
		Values:  map[string]any{"replicas": 2},
	}
	revs := []revision.Revision{
		{Name: "rev-a", Created: time.Unix(1, 0), OCI: "oci://demo", Version: "1.0.0", Values: map[string]any{"replicas": 2}},
		{Name: "rev-b", Created: time.Unix(2, 0), OCI: "oci://demo", Version: "1.0.0", Values: map[string]any{"replicas": 9}},
	}
	return claim, revs, demoPuller(t), eng
}

func sizedRev(m revModel) revModel {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(revModel)
}

// Selecting a revision whose values differ from the claim yields a diff with
// changes, delivered via revDiffMsg, and populates the columns.
func TestSelectRevisionProducesDiff(t *testing.T) {
	claim, revs, puller, eng := testFixtures(t)
	m := sizedRev(newRevModel(claim, revs, puller, eng))

	// navigate to rev-b (index 1) so the selection matches the command
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(revModel)
	cmd := m.selectCmd(m.selected) // rev-b: replicas 9 vs claim replicas 2 -> a change
	if cmd == nil {
		t.Fatal("expected a command for an uncached revision")
	}
	result := cmd()
	msg, ok := result.(revDiffMsg)
	if !ok {
		t.Fatalf("command did not return revDiffMsg, got %T", result)
	}
	if msg.err != nil {
		t.Fatalf("diff error: %v", msg.err)
	}
	if msg.name != "rev-b" {
		t.Errorf("msg.name = %q, want rev-b", msg.name)
	}
	added, removed := msg.result.Counts()
	if added == 0 && removed == 0 {
		t.Error("expected replicas change to produce a diff")
	}

	updated, _ = m.Update(msg)
	m = updated.(revModel)
	view := m.View().Content
	if !strings.Contains(view, "replicas: 9") {
		t.Errorf("revision column should show replicas: 9\n%s", view)
	}
	if !strings.Contains(view, "replicas: 2") {
		t.Errorf("claim column should show replicas: 2\n%s", view)
	}
}

// Re-selecting a cached revision returns no command (instant).
func TestReselectCachedRevisionSkipsCommand(t *testing.T) {
	claim, revs, puller, eng := testFixtures(t)
	m := sizedRev(newRevModel(claim, revs, puller, eng))

	cmd := m.selectCmd(1)
	msg := cmd().(revDiffMsg)
	updated, _ := m.Update(msg)
	m = updated.(revModel)

	if again := m.selectCmd(1); again != nil {
		t.Error("re-selecting a cached revision should return nil command")
	}
}

// The revision list is shown and quit keys work.
func TestRevViewListsRevisionsAndQuits(t *testing.T) {
	claim, revs, puller, eng := testFixtures(t)
	m := sizedRev(newRevModel(claim, revs, puller, eng))

	view := m.View().Content
	if !strings.Contains(view, "rev-a") || !strings.Contains(view, "rev-b") {
		t.Errorf("revision list not shown:\n%s", view)
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q did not map to tea.Quit")
	}
}

// A pull failure surfaces as a revDiffMsg carrying the error, not a panic.
func TestSelectRevisionSurfacesError(t *testing.T) {
	claim, revs, _, eng := testFixtures(t)
	m := sizedRev(newRevModel(claim, revs, errPuller{}, eng))

	cmd := m.selectCmd(1)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg, ok := cmd().(revDiffMsg)
	if !ok {
		t.Fatalf("want revDiffMsg, got %T", cmd())
	}
	if msg.err == nil {
		t.Error("expected pull error to surface in revDiffMsg")
	}
}

// Pressing up at the first revision (or down at the last) must not re-fire a
// render command for the unchanged selection.
func TestNavigationAtBoundaryIsNoOp(t *testing.T) {
	claim, revs, puller, eng := testFixtures(t)
	m := sizedRev(newRevModel(claim, revs, puller, eng)) // selected = 0

	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"}); cmd != nil {
		t.Error("up at first revision should be a no-op, got a command")
	}

	// move to the last revision, then down again is a no-op
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(revModel)
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"}); cmd != nil {
		t.Error("down at last revision should be a no-op, got a command")
	}
}

// Navigating away from a revision that errored to a cached, successful one must
// clear the stale error so the good diff is shown (regression).
func TestNavigatingAwayFromErrorClearsIt(t *testing.T) {
	claim, _, _, eng := testFixtures(t)
	puller := versionPuller{ch: demoPuller(t).ch, failVersion: "9.9.9"}
	revs := []revision.Revision{
		{Name: "rev-a", Created: time.Unix(1, 0), OCI: "oci://demo", Version: "1.0.0", Values: map[string]any{"replicas": 5}},
		{Name: "rev-b", Created: time.Unix(2, 0), OCI: "oci://demo", Version: "9.9.9", Values: map[string]any{"replicas": 7}},
	}
	m := sizedRev(newRevModel(claim, revs, puller, eng))

	// Cache rev-a (renders fine), staying on index 0.
	updated, _ := m.Update(m.selectCmd(0)().(revDiffMsg))
	m = updated.(revModel)

	// Navigate down to rev-b, whose chart version fails to pull.
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(revModel)
	if cmd == nil {
		t.Fatal("expected a command for uncached rev-b")
	}
	updated, _ = m.Update(cmd().(revDiffMsg)) // delivers the error
	m = updated.(revModel)
	if m.err == nil {
		t.Fatal("precondition: rev-b should have surfaced an error")
	}

	// Navigate back up to the cached, good rev-a.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = updated.(revModel)

	view := m.View().Content
	if strings.Contains(view, "boom") {
		t.Errorf("stale error still shown after navigating to a good revision:\n%s", view)
	}
	if !strings.Contains(view, "replicas: 5") {
		t.Errorf("good revision's diff not shown:\n%s", view)
	}
}

// countPuller records how many times each ref was pulled. selectCmd's command
// runs synchronously in these tests, so no locking is needed.
type countPuller struct {
	ch    *chart.Chart
	calls map[string]int
}

func (p *countPuller) Pull(ref string) (*chart.Chart, error) {
	p.calls[ref]++
	return p.ch, nil
}

// The model must wrap its puller in oci.NewCachingPuller: rev-a shares the
// claim's ref (oci://demo:1.0.0), so rendering both sides of one diff pulls the
// shared chart exactly once.
func TestModelCachesSharedChartPulls(t *testing.T) {
	claim, revs, _, eng := testFixtures(t)
	inner := &countPuller{ch: demoPuller(t).ch, calls: map[string]int{}}
	m := sizedRev(newRevModel(claim, revs, inner, eng))

	m.selectCmd(0)() // rev-a: claim ref == rev ref

	if got := inner.calls["oci://demo:1.0.0"]; got != 1 {
		t.Errorf("shared ref pulled %d times, want 1 (model must wrap puller in a caching puller)", got)
	}
}

// fixedNow is a stable clock for approval rendering tests.
var fixedNow = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

// approvalModel builds a sized revision model with a pinned clock and three
// revisions: approved (past), unapproved (nil), and future-approved.
func approvalModel(t *testing.T) revModel {
	t.Helper()
	_, _, puller, eng := testFixtures(t)
	claim := revision.Claim{OCI: "oci://demo", Version: "1.0.0", Values: map[string]any{"replicas": 2}}
	past := time.Date(2026, 7, 8, 16, 1, 0, 0, time.UTC)
	future := time.Date(2026, 7, 20, 16, 1, 0, 0, time.UTC)
	revs := []revision.Revision{
		{Name: "rev-approved", Created: time.Unix(1, 0), OCI: "oci://demo", Version: "1.0.0", ApprovedAt: &past},
		{Name: "rev-unapproved", Created: time.Unix(2, 0), OCI: "oci://demo", Version: "1.0.0", ApprovedAt: nil},
		{Name: "rev-future", Created: time.Unix(3, 0), OCI: "oci://demo", Version: "1.0.0", ApprovedAt: &future},
	}
	m := newRevModel(claim, revs, puller, eng)
	m.now = fixedNow
	// Wide enough that the full "approved at: …" status line fits the list
	// column without truncation (listColMax = 34).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return updated.(revModel)
}

func TestListShowsApprovalStatusLines(t *testing.T) {
	view := approvalModel(t).View().Content
	for _, want := range []string{
		"rev-approved",
		"approved at: 2026-07-08 16:01",
		"rev-unapproved",
		"not approved",
		"rev-future",
		"approved at: 2026-07-20 16:01",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("list view missing %q\n%s", want, view)
		}
	}
}

type errPuller struct{}

func (errPuller) Pull(string) (*chart.Chart, error) { return nil, errBoom }

// versionPuller returns the chart for any ref except one whose tag equals
// failVersion, for which it returns an error.
type versionPuller struct {
	ch          *chart.Chart
	failVersion string
}

func (p versionPuller) Pull(ref string) (*chart.Chart, error) {
	if strings.HasSuffix(ref, ":"+p.failVersion) {
		return nil, errBoom
	}
	return p.ch, nil
}

var errBoom = boomError("boom")

type boomError string

func (e boomError) Error() string { return string(e) }
