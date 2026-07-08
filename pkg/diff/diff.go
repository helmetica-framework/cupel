// Package diff computes and represents differences between two rendered
// Helm charts. Engines self-register and emit layout-agnostic aligned rows.
package diff

import "fmt"

// registry holds the diff engines keyed by name. It is populated by Register,
// typically from engine init() functions, and read by Get.
var registry = map[string]func() Engine{}

// Rendered is one side of a comparison: a chart's fully rendered manifests
// plus the OCI reference it came from, used for column labels in the UI.
type Rendered struct {
	Ref      string
	Manifest string
}

// CellKind describes how a single cell relates to its counterpart on the
// other side of the diff.
type CellKind int

const (
	// Same marks a line present and unchanged on both sides.
	Same CellKind = iota
	// Added marks a line present only on the right (the "after") side.
	Added
	// Removed marks a line present only on the left (the "before") side.
	Removed
	// Pad marks a blank filler cell that keeps the opposite column aligned
	// when the two sides have an unequal number of lines.
	Pad
)

// Engine computes the diff between two rendered charts. Implementations own
// how they pair lines into rows, so the presentation layer stays trivial.
type Engine interface {
	// Name returns the engine's registry name.
	Name() string
	// Diff compares a (before) against b (after) and returns aligned rows.
	Diff(a, b Rendered) (Result, error)
}

// Cell is one side of a Row: its relationship to the other side (Kind) and
// the line text it displays.
type Cell struct {
	Kind CellKind
	Text string
}

// Row is a single aligned line of the diff, pairing the left (before) and
// right (after) cells so they render on the same terminal row.
type Row struct {
	Left  Cell
	Right Cell
}

// Result is an engine's output: an ordered, layout-agnostic list of aligned
// rows that a side-by-side or inline view can both render.
type Result struct {
	Rows []Row
}

// Counts reports how many lines were added and removed across the diff. A
// changed line contributes to both totals, since it is a removal paired with
// an addition.
func (r Result) Counts() (added, removed int) {
	for _, row := range r.Rows {
		if row.Right.Kind == Added {
			added++
		}
		if row.Left.Kind == Removed {
			removed++
		}
	}
	return
}

// Register adds a diff engine under the given name. The last registration for
// a name wins.
func Register(name string, factory func() Engine) {
	registry[name] = factory
}

// Get returns a freshly constructed engine for the given name, or an error if
// no engine is registered under it.
func Get(name string) (Engine, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown diff engine %q", name)
	}
	return factory(), nil
}
