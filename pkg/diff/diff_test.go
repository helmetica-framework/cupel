package diff

import "testing"

func TestCountsCountsAddedAndRemovedRows(t *testing.T) {
	r := Result{Rows: []Row{
		{Left: Cell{Same, "a"}, Right: Cell{Same, "a"}},
		{Left: Cell{Removed, "b"}, Right: Cell{Added, "B"}},
		{Left: Cell{Pad, ""}, Right: Cell{Added, "c"}},
		{Left: Cell{Removed, "d"}, Right: Cell{Pad, ""}},
	}}
	added, removed := r.Counts()
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
}

func TestRegistryGetReturnsRegisteredEngine(t *testing.T) {
	Register("fake", func() Engine { return fakeEngine{} })
	got, err := Get("fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != "fake" {
		t.Errorf("Name() = %q, want %q", got.Name(), "fake")
	}
}

func TestRegistryGetUnknownReturnsError(t *testing.T) {
	if _, err := Get("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown engine, got nil")
	}
}

type fakeEngine struct{}

func (fakeEngine) Name() string                       { return "fake" }
func (fakeEngine) Diff(a, b Rendered) (Result, error) { return Result{}, nil }
