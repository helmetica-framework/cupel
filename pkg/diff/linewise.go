package diff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// linewiseEngine diffs two manifests line by line and pairs the changed
// lines into aligned rows for a side-by-side view.
type linewiseEngine struct{}

func init() {
	Register("linewise", func() Engine { return linewiseEngine{} })
}

// Name returns the engine's registry name.
func (linewiseEngine) Name() string { return "linewise" }

// Diff compares a (before) against b (after) line by line. Consecutive
// removals and additions are paired into single rows; a surplus on either
// side is filled with Pad cells so the two columns stay aligned.
func (linewiseEngine) Diff(a, b Rendered) (Result, error) {
	dmp := diffmatchpatch.New()

	// Encode each distinct line as a rune so the diff works line-by-line
	// instead of character-by-character, then decode back to lines.
	c1, c2, lines := dmp.DiffLinesToChars(a.Manifest, b.Manifest)
	diffs := dmp.DiffMain(c1, c2, false)
	diffs = dmp.DiffCharsToLines(diffs, lines)

	var rows []Row
	var dels, inss []string // pending removed / added lines

	// flush pairs the buffered removals and additions into rows, padding the
	// shorter side, then clears the buffers.
	flush := func() {
		n := max(len(dels), len(inss))
		for i := range n {
			left := Cell{Kind: Pad}
			if i < len(dels) {
				left = Cell{Kind: Removed, Text: dels[i]}
			}
			right := Cell{Kind: Pad}
			if i < len(inss) {
				right = Cell{Kind: Added, Text: inss[i]}
			}
			rows = append(rows, Row{Left: left, Right: right})
		}
		dels, inss = nil, nil
	}

	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			// flush any pending change block before the equal run
			flush()
			for _, line := range splitLines(d.Text) {
				rows = append(rows, Row{
					Left:  Cell{Kind: Same, Text: line},
					Right: Cell{Kind: Same, Text: line},
				})
			}
		case diffmatchpatch.DiffDelete:
			dels = append(dels, splitLines(d.Text)...)
		case diffmatchpatch.DiffInsert:
			inss = append(inss, splitLines(d.Text)...)
		}
	}
	flush() // flush the trailing change block, if any

	return Result{Rows: rows}, nil
}

// splitLines splits a diff chunk into its lines, dropping the trailing empty
// element that a terminating newline would otherwise produce.
func splitLines(chunk string) []string {
	if chunk == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(chunk, "\n"), "\n")
}
