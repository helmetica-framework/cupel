package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/helmetica-framework/cupel/pkg/diff"
)

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

func TestEmptyDiffShowsIdenticalMessage(t *testing.T) {
	m := sized(New("a", "b", diff.Result{Rows: []diff.Row{
		{Left: diff.Cell{Kind: diff.Same, Text: "x"}, Right: diff.Cell{Kind: diff.Same, Text: "x"}},
	}}))
	if !strings.Contains(strings.ToLower(m.View()), "identical") {
		t.Errorf("expected identical message for zero-change diff:\n%s", m.View())
	}
}
