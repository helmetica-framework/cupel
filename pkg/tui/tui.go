// Package tui provides a Bubble Tea side-by-side diff viewer for cupel.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/helmetica-framework/cupel/pkg/diff"
)

const headerHeight = 2

var (
	styleRemoved = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleAdded   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleSame    = lipgloss.NewStyle().Faint(true)
	stylePlus    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleMinus   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// Model is the Bubble Tea model for the side-by-side diff viewer.
type Model struct {
	refA, refB string
	result     diff.Result
	left       viewport.Model
	right      viewport.Model
	ready      bool
	width      int
}

// New creates a new Model for the given chart references and diff result.
func New(refA, refB string, result diff.Result) Model {
	return Model{
		refA:   refA,
		refB:   refB,
		result: result,
	}
}

// Init satisfies tea.Model; it returns nil because no initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages, resizing viewports and forwarding scroll
// keys to both columns so they scroll in lockstep.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		colW := max((msg.Width-3)/2, 1)
		colH := max(msg.Height-headerHeight, 1)
		if !m.ready {
			m.left = viewport.New(colW, colH)
			m.right = viewport.New(colW, colH)
			m.ready = true
		} else {
			m.left.Width = colW
			m.left.Height = colH
			m.right.Width = colW
			m.right.Height = colH
		}
		leftContent, rightContent := m.renderRows(colW)
		m.left.SetContent(leftContent)
		m.right.SetContent(rightContent)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
		var cmdL, cmdR tea.Cmd
		m.left, cmdL = m.left.Update(msg)
		m.right, cmdR = m.right.Update(msg)
		return m, tea.Batch(cmdL, cmdR)
	}

	return m, nil
}

// renderRows builds the left and right column content strings from the diff
// rows, applying Lip Gloss styles and truncating long lines.
func (m Model) renderRows(colW int) (leftOut, rightOut string) {
	var leftLines, rightLines []string
	for _, row := range m.result.Rows {
		leftLines = append(leftLines, styledCell(row.Left, colW))
		rightLines = append(rightLines, styledCell(row.Right, colW))
	}
	return strings.Join(leftLines, "\n"), strings.Join(rightLines, "\n")
}

// styledCell renders a single cell with the appropriate Lip Gloss style and
// truncates the text to fit within the column width.
func styledCell(c diff.Cell, colW int) string {
	if c.Kind == diff.Pad {
		return ""
	}
	text := c.Text
	// Reserve 2 chars for the gutter marker and a space.
	maxText := max(colW-2, 1)
	if len([]rune(text)) > maxText {
		runes := []rune(text)
		text = string(runes[:maxText-1]) + "…"
	}
	var marker string
	var st lipgloss.Style
	switch c.Kind {
	case diff.Removed:
		marker = "-"
		st = styleRemoved
	case diff.Added:
		marker = "+"
		st = styleAdded
	default:
		marker = " "
		st = styleSame
	}
	return st.Render(marker + " " + text)
}

// View renders the full TUI, returning "initializing…" until the first
// WindowSizeMsg has been processed.
func (m Model) View() string {
	if !m.ready {
		return "initializing…"
	}

	added, removed := m.result.Counts()

	header := fmt.Sprintf("%s → %s  %s %s",
		m.refA,
		m.refB,
		stylePlus.Render(fmt.Sprintf("+%d", added)),
		styleMinus.Render(fmt.Sprintf("-%d", removed)),
	)

	if added == 0 && removed == 0 {
		identical := lipgloss.Place(
			m.width, m.left.Height,
			lipgloss.Center, lipgloss.Center,
			"charts are identical",
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, identical)
	}

	gutter := " "
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		m.left.View(),
		gutter,
		m.right.View(),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, columns)
}

// Run starts a new Bubble Tea program with the alt-screen buffer and blocks
// until the user quits.
func Run(refA, refB string, result diff.Result) error {
	p := tea.NewProgram(New(refA, refB, result), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
