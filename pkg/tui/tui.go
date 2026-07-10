// Package tui provides a Bubble Tea side-by-side diff viewer for cupel.
package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/helmetica-framework/cupel/pkg/diff"
)

// headerHeight is the vertical space reserved for the header line plus one
// blank line of margin; it is subtracted from the terminal height when sizing
// the viewports.
const headerHeight = 2

var (
	styleRemoved = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleAdded   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleSame    = lipgloss.NewStyle().Faint(true)
	stylePlus    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleMinus   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	styleApproved   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	styleUnapproved = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	styleFuture     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
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
			m.left = viewport.New(viewport.WithWidth(colW), viewport.WithHeight(colH))
			m.right = viewport.New(viewport.WithWidth(colW), viewport.WithHeight(colH))
			m.ready = true
		} else {
			m.left.SetWidth(colW)
			m.left.SetHeight(colH)
			m.right.SetWidth(colW)
			m.right.SetHeight(colH)
		}
		leftContent, rightContent := m.renderRows(colW)
		m.left.SetContent(leftContent)
		m.right.SetContent(rightContent)
		return m, nil

	case tea.KeyPressMsg:
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

// buildColumns builds the left and right column content strings from the diff
// rows, applying Lip Gloss styles and truncating long lines.
func buildColumns(rows []diff.Row, colW int) (left, right string) {
	var leftLines, rightLines []string
	for _, row := range rows {
		leftLines = append(leftLines, styledCell(row.Left, colW))
		rightLines = append(rightLines, styledCell(row.Right, colW))
	}
	return strings.Join(leftLines, "\n"), strings.Join(rightLines, "\n")
}

// renderRows delegates to the package-level buildColumns helper.
func (m Model) renderRows(colW int) (leftOut, rightOut string) {
	return buildColumns(m.result.Rows, colW)
}

// styledCell renders a single cell with the appropriate Lip Gloss style and
// truncates the text to fit within the column width.
func styledCell(c diff.Cell, colW int) string {
	if c.Kind == diff.Pad {
		return ""
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
	// Hard-clamp the whole line (gutter marker + space + text) to the column
	// width in display cells, appending an ellipsis when truncated. ansi.Truncate
	// is width-aware, so wide/multibyte runes never overflow the column.
	line := ansi.Truncate(marker+" "+c.Text, colW, "…")
	return st.Render(line)
}

// View renders the full TUI as an alt-screen Bubble Tea view. In v2 the
// alt-screen buffer is requested on the View rather than as a program option.
func (m Model) View() tea.View {
	v := tea.NewView(m.content())
	v.AltScreen = true
	return v
}

// content builds the rendered screen text, showing "initializing…" until the
// first WindowSizeMsg has been processed.
func (m Model) content() string {
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
	// Keep the header on one line so it can't wrap and desync the columns.
	header = ansi.Truncate(header, m.width, "…")

	if added == 0 && removed == 0 {
		identical := lipgloss.Place(
			m.width, m.left.Height(),
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

// Run starts a new Bubble Tea program and blocks until the user quits. The
// alt-screen buffer is enabled via the model's View.
func Run(refA, refB string, result diff.Result) error {
	p := tea.NewProgram(New(refA, refB, result))
	_, err := p.Run()
	return err
}
