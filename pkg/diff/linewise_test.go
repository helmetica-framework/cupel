package diff

import "testing"

// rendered wraps a manifest string in a Rendered with a fixed ref, so tests
// can pass raw text to an engine's Diff.
func rendered(manifest string) Rendered {
	return Rendered{Ref: "test", Manifest: manifest}
}

// wantRow asserts that a row's left and right cells each have the expected
// kind and text.
func wantRow(t *testing.T, got Row, lk CellKind, lt string, rk CellKind, rt string) {
	t.Helper()
	if got.Left.Kind != lk || got.Left.Text != lt {
		t.Errorf("left = {%v %q}, want {%v %q}", got.Left.Kind, got.Left.Text, lk, lt)
	}
	if got.Right.Kind != rk || got.Right.Text != rt {
		t.Errorf("right = {%v %q}, want {%v %q}", got.Right.Kind, got.Right.Text, rk, rt)
	}
}

// mustEngine fetches the named engine, failing the test if it is not
// registered.
func mustEngine(t *testing.T, name string) Engine {
	t.Helper()
	eng, err := Get(name)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	return eng
}

// Identical inputs yield only Same|Same rows and a zero change count.
func TestLinewiseIdenticalInputsAllSame(t *testing.T) {
	eng := mustEngine(t, "linewise")
	in := "a\nb\nc\n"
	res, err := eng.Diff(rendered(in), rendered(in))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(res.Rows))
	}
	for i, row := range res.Rows {
		if row.Left.Kind != Same || row.Right.Kind != Same {
			t.Errorf("row %d not Same/Same: %+v", i, row)
		}
	}
	added, removed := res.Counts()
	if added != 0 || removed != 0 {
		t.Errorf("counts = (%d,%d), want (0,0)", added, removed)
	}
}

// A single changed line becomes one Removed|Added row, flanked by Same rows.
func TestLinewiseChangedLinePairsRemovedAndAdded(t *testing.T) {
	eng := mustEngine(t, "linewise")
	res, err := eng.Diff(rendered("a\nb\nc\n"), rendered("a\nB\nc\n"))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("rows = %d, want 3: %+v", len(res.Rows), res.Rows)
	}
	wantRow(t, res.Rows[0], Same, "a", Same, "a")
	wantRow(t, res.Rows[1], Removed, "b", Added, "B")
	wantRow(t, res.Rows[2], Same, "c", Same, "c")
}

// A line present only on the right pairs with a Pad cell on the left.
func TestLinewisePureAddPadsLeft(t *testing.T) {
	eng := mustEngine(t, "linewise")
	res, err := eng.Diff(rendered("a\nc\n"), rendered("a\nb\nc\n"))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	// Expect a row where left is Pad and right is Added "b".
	var found bool
	for _, row := range res.Rows {
		if row.Left.Kind == Pad && row.Right.Kind == Added && row.Right.Text == "b" {
			found = true
		}
	}
	if !found {
		t.Errorf("no Pad|Added(b) row found: %+v", res.Rows)
	}
	added, removed := res.Counts()
	if added != 1 || removed != 0 {
		t.Errorf("counts = (%d,%d), want (1,0)", added, removed)
	}
}

// A line present only on the left pairs with a Pad cell on the right.
func TestLinewisePureRemovePadsRight(t *testing.T) {
	eng := mustEngine(t, "linewise")
	res, err := eng.Diff(rendered("a\nb\nc\n"), rendered("a\nc\n"))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var found bool
	for _, row := range res.Rows {
		if row.Left.Kind == Removed && row.Left.Text == "b" && row.Right.Kind == Pad {
			found = true
		}
	}
	if !found {
		t.Errorf("no Removed(b)|Pad row found: %+v", res.Rows)
	}
	added, removed := res.Counts()
	if added != 0 || removed != 1 {
		t.Errorf("counts = (%d,%d), want (0,1)", added, removed)
	}
}

// When the removed and added runs differ in length, the surplus lines pair
// with Pad cells so the columns stay aligned.
func TestLinewiseUnequalRunsSurplusPads(t *testing.T) {
	eng := mustEngine(t, "linewise")
	// two removed lines, one added line -> one paired row + one Removed|Pad row.
	res, err := eng.Diff(rendered("x\ny\n"), rendered("Z\n"))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	added, removed := res.Counts()
	if added != 1 || removed != 2 {
		t.Errorf("counts = (%d,%d), want (1,2): %+v", added, removed, res.Rows)
	}
}
