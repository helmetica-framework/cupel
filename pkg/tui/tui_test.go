package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/helmetica-framework/cupel/pkg/diff"
)

// A rendered cell must never exceed its column width in display cells, even on
// very narrow terminals or with wide/multibyte text (regression: the gutter
// marker used to overflow columns of width <= 2).
func TestStyledCellNeverExceedsColumnWidth(t *testing.T) {
	cells := []diff.Cell{
		{Kind: diff.Removed, Text: "image: app:1.2.0 with a long trailing description"},
		{Kind: diff.Added, Text: "日本語テキストのとても長い行"},
		{Kind: diff.Same, Text: "apiVersion: v1"},
	}
	for _, colW := range []int{1, 2, 3, 5, 12, 40} {
		for _, c := range cells {
			if w := ansi.StringWidth(styledCell(c, colW)); w > colW {
				t.Errorf("colW=%d, kind=%v: width %d exceeds column", colW, c.Kind, w)
			}
		}
	}
}

func sampleResult() diff.Result {
	return diff.Result{Rows: []diff.Row{
		{Left: diff.Cell{Kind: diff.Same, Text: "apiVersion: v1"}, Right: diff.Cell{Kind: diff.Same, Text: "apiVersion: v1"}},
		{Left: diff.Cell{Kind: diff.Removed, Text: "image: app:1.2.0"}, Right: diff.Cell{Kind: diff.Added, Text: "image: app:1.3.0"}},
	}}
}

// send a WindowSizeMsg so viewports get a size before rendering.
func sized(m Model) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return updated.(Model)
}

func TestViewShowsHeaderRefsAndCounts(t *testing.T) {
	m := sized(New("oci://a:1.2.0", "oci://b:1.3.0", sampleResult()))
	view := m.View()
	if !strings.Contains(view, "oci://a:1.2.0") || !strings.Contains(view, "oci://b:1.3.0") {
		t.Errorf("header missing refs:\n%s", view)
	}
	if !strings.Contains(view, "+1") || !strings.Contains(view, "-1") {
		t.Errorf("header missing +1/-1 summary:\n%s", view)
	}
}

func TestViewRendersBothSidesText(t *testing.T) {
	m := sized(New("a", "b", sampleResult()))
	view := m.View()
	if !strings.Contains(view, "image: app:1.2.0") {
		t.Errorf("left content missing:\n%s", view)
	}
	if !strings.Contains(view, "image: app:1.3.0") {
		t.Errorf("right content missing:\n%s", view)
	}
}

func TestQuitKeyReturnsQuitCmd(t *testing.T) {
	m := sized(New("a", "b", sampleResult()))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	// tea.Quit returns a tea.QuitMsg when invoked.
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q did not map to tea.Quit")
	}
}

// A zero-change diff — whether every row is Same or there are no rows at all —
// shows the centered "charts are identical" message.
func TestEmptyDiffShowsIdenticalMessage(t *testing.T) {
	cases := map[string]diff.Result{
		"all same": {Rows: []diff.Row{
			{Left: diff.Cell{Kind: diff.Same, Text: "x"}, Right: diff.Cell{Kind: diff.Same, Text: "x"}},
		}},
		"no rows": {},
	}
	for name, res := range cases {
		t.Run(name, func(t *testing.T) {
			m := sized(New("a", "b", res))
			if !strings.Contains(strings.ToLower(m.View()), "identical") {
				t.Errorf("expected identical message:\n%s", m.View())
			}
		})
	}
}
