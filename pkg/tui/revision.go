package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/revision"
	"github.com/helmetica-framework/cupel/pkg/source"
)

// revDiffMsg carries the outcome of an asynchronous revision render+diff back
// to Update, keyed by revision name so Update can cache it and decide whether
// it is still the selected revision.
type revDiffMsg struct {
	name   string
	result diff.Result
	err    error
}

// revModel is the Bubble Tea model for the revision-diff view: a list of
// revisions on the left and the claim-vs-selected-revision side-by-side diff on
// the right. Diffs are rendered lazily on selection and cached.
type revModel struct {
	claim    revision.Claim
	revs     []revision.Revision
	puller   oci.Puller
	engine   diff.Engine
	selected int
	loading  bool
	err      error

	diffCache map[string]diff.Result // rendered diffs, by revision name

	left, right   viewport.Model
	width, height int
	colW          int
	ready         bool
}

// newRevModel creates a revModel for the given claim base and revisions. The
// puller is wrapped in a caching puller so charts shared across the claim and
// its revisions (same oci:version) are pulled once, even across the tea.Cmd
// goroutines that render each revision.
func newRevModel(claim revision.Claim, revs []revision.Revision, puller oci.Puller, engine diff.Engine) revModel {
	return revModel{
		claim:     claim,
		revs:      revs,
		puller:    oci.NewCachingPuller(puller),
		engine:    engine,
		selected:  0,
		diffCache: map[string]diff.Result{},
	}
}

// revisionDiff renders the claim base and one revision through the source
// abstraction and diffs claim (before) against revision (after). puller is the
// model's caching puller, so shared charts are pulled once; it is safe to run
// inside a tea.Cmd goroutine.
func revisionDiff(puller oci.Puller, engine diff.Engine, claim revision.Claim, rev revision.Revision) (diff.Result, error) {
	claimMan, claimLabel, err := source.Claim(claim).Render(puller)
	if err != nil {
		return diff.Result{}, fmt.Errorf("claim: %w", err)
	}

	revMan, revLabel, err := source.Revision(rev).Render(puller)
	if err != nil {
		return diff.Result{}, fmt.Errorf("revision %s: %w", rev.Name, err)
	}

	return engine.Diff(diff.Rendered{
		Ref:      claimLabel,
		Manifest: claimMan,
	}, diff.Rendered{
		Ref:      revLabel,
		Manifest: revMan,
	})
}

// selectCmd returns the command that computes the diff for the revision at
// index, or nil if it is already cached (the caller renders the cached result
// directly). The returned command captures everything it needs by value, so it
// never touches model state from its goroutine; Update caches the result when
// the resulting revDiffMsg arrives.
func (m revModel) selectCmd(index int) tea.Cmd {
	rev := m.revs[index]
	if _, ok := m.diffCache[rev.Name]; ok {
		return nil
	}

	claim, puller, engine := m.claim, m.puller, m.engine
	return func() tea.Msg {
		res, err := revisionDiff(puller, engine, claim, rev)
		return revDiffMsg{name: rev.Name, result: res, err: err}
	}
}

// Init auto-selects the first revision so its diff renders as the view opens,
// or does nothing when there are no revisions.
func (m revModel) Init() tea.Cmd {
	if len(m.revs) == 0 {
		return nil
	}
	return m.selectCmd(0)
}

// applySelection moves the selection to index and returns the command that
// renders it, or nil when the result is already cached (repainted immediately)
// or when index is out of range or unchanged. An out-of-range index (list
// boundary) or a no-op reselection does nothing, avoiding a duplicate render.
func (m *revModel) applySelection(index int) tea.Cmd {
	if index < 0 || index >= len(m.revs) || index == m.selected {
		return nil
	}
	m.selected = index

	if cmd := m.selectCmd(index); cmd != nil {
		m.loading = true
		return cmd
	}
	// Already cached: repaint now and clear any prior revision's error.
	m.loading = false
	m.err = nil
	m.repaintSelected()
	return nil
}

// repaintSelected refills the diff columns from the selected revision's cached
// result, if one exists and the viewports are sized.
func (m *revModel) repaintSelected() {
	if !m.ready || len(m.revs) == 0 {
		return
	}
	if cached, ok := m.diffCache[m.revs[m.selected].Name]; ok {
		l, r := buildColumns(cached.Rows, m.colW)
		m.left.SetContent(l)
		m.right.SetContent(r)
	}
}

// Update handles resize, key navigation over the revision list, and incoming
// revDiffMsg results. Selecting a revision renders it (or repaints from cache);
// scroll keys drive both diff viewports in lockstep.
func (m revModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listW := min(28, msg.Width/4)
		m.colW = max((msg.Width-listW-3)/2, 1)
		colH := max(msg.Height-headerHeight, 1)
		if !m.ready {
			m.left = viewport.New(viewport.WithWidth(m.colW), viewport.WithHeight(colH))
			m.right = viewport.New(viewport.WithWidth(m.colW), viewport.WithHeight(colH))
			m.ready = true
		} else {
			m.left.SetWidth(m.colW)
			m.left.SetHeight(colH)
			m.right.SetWidth(m.colW)
			m.right.SetHeight(colH)
		}
		m.repaintSelected()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			return m, (&m).applySelection(m.selected - 1)
		case "down", "j":
			return m, (&m).applySelection(m.selected + 1)
		default:
			var cmdL, cmdR tea.Cmd
			m.left, cmdL = m.left.Update(msg)
			m.right, cmdR = m.right.Update(msg)
			return m, tea.Batch(cmdL, cmdR)
		}

	case revDiffMsg:
		m.diffCache[msg.name] = msg.result
		// Repaint only when this diff is for the currently selected revision.
		if len(m.revs) > 0 && msg.name == m.revs[m.selected].Name {
			m.err = msg.err
			m.loading = false
			if msg.err == nil {
				m.repaintSelected()
			}
		}
		return m, nil
	}

	return m, nil
}

// View renders the model as an alt-screen Bubble Tea view.
func (m revModel) View() tea.View {
	v := tea.NewView(m.revContent())
	v.AltScreen = true
	return v
}

// revContent builds the screen text: the revision list column, a header with
// the claim ref, selected revision, and change counts, and the diff area
// (a "rendering…" or error placeholder, otherwise the two viewports).
func (m revModel) revContent() string {
	if !m.ready {
		return "initializing…"
	}

	listW := min(28, m.width/4)

	// Build the revision list column.
	var listLines []string
	for i, rev := range m.revs {
		prefix := "  "
		if i == m.selected {
			prefix = "▸ "
		}
		line := ansi.Truncate(prefix+rev.Name, listW, "…")
		if i != m.selected {
			line = lipgloss.NewStyle().Faint(true).Render(line)
		}
		listLines = append(listLines, line)
	}
	listColumn := strings.Join(listLines, "\n")

	// Build the header.
	selName := ""
	var added, removed int
	if len(m.revs) > 0 {
		selName = m.revs[m.selected].Name
		if cached, ok := m.diffCache[selName]; ok {
			added, removed = cached.Counts()
		}
	}
	header := fmt.Sprintf(
		"%s  %s  %s %s",
		m.claim.OCI,
		selName,
		stylePlus.Render(fmt.Sprintf("+%d", added)),
		styleMinus.Render(fmt.Sprintf("-%d", removed)),
	)
	header = ansi.Truncate(header, m.width, "…")

	// Build the diff area.
	var diffArea string
	switch {
	case m.loading:
		diffArea = "rendering…"
	case m.err != nil:
		diffArea = m.err.Error()
	default:
		diffArea = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.left.View(),
			" ",
			m.right.View(),
		)
	}

	columns := lipgloss.JoinHorizontal(lipgloss.Top, listColumn, " ", diffArea)
	return lipgloss.JoinVertical(lipgloss.Left, header, columns)
}

// RunRevisions starts the revision-diff TUI and blocks until the user quits.
func RunRevisions(claim revision.Claim, revs []revision.Revision, puller oci.Puller, engine diff.Engine) error {
	_, err := tea.NewProgram(newRevModel(claim, revs, puller, engine)).Run()
	return err
}
