package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmetica-framework/cupel/pkg/oci"
)

// fakePuller returns preloaded charts keyed by ref, implementing oci.Puller.
type fakePuller struct {
	charts map[string]*chart.Chart
	err    error
}

func (f fakePuller) Pull(ref string) (*chart.Chart, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.charts[ref], nil
}

var _ oci.Puller = fakePuller{}

func loadChart(t *testing.T, path string) *chart.Chart {
	t.Helper()
	raw, err := loader.Load(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	ch, ok := raw.(*chart.Chart)
	if !ok {
		t.Fatalf("%s is not *chart.Chart", path)
	}
	return ch
}

func TestComputeDiffIdenticalChartsHaveNoChanges(t *testing.T) {
	ch := loadChart(t, "../pkg/render/testdata/demo")
	p := fakePuller{charts: map[string]*chart.Chart{"a": ch, "b": ch}}
	res, err := computeDiff(p, "linewise", "a", "b")
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	added, removed := res.Counts()
	if added != 0 || removed != 0 {
		t.Errorf("counts = (%d,%d), want (0,0)", added, removed)
	}
	if len(res.Rows) == 0 {
		t.Error("expected rows for a non-empty render")
	}
}

func TestComputeDiffUnknownEngineErrors(t *testing.T) {
	ch := loadChart(t, "../pkg/render/testdata/demo")
	p := fakePuller{charts: map[string]*chart.Chart{"a": ch, "b": ch}}
	if _, err := computeDiff(p, "nope", "a", "b"); err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestComputeDiffPropagatesPullError(t *testing.T) {
	p := fakePuller{err: errBoom}
	if _, err := computeDiff(p, "linewise", "a", "b"); err == nil {
		t.Fatal("expected pull error to propagate")
	} else if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want it to mention boom", err)
	}
}

func TestComputeDiffPropagatesRenderError(t *testing.T) {
	// A chart whose template calls the `fail` function cannot render.
	bad := &chart.Chart{
		Metadata:  &chart.Metadata{APIVersion: "v2", Name: "bad", Version: "0.1.0"},
		Templates: []*common.File{{Name: "templates/x.yaml", Data: []byte(`{{ fail "boom" }}`)}},
	}
	p := fakePuller{charts: map[string]*chart.Chart{"a": bad, "b": bad}}
	if _, err := computeDiff(p, "linewise", "a", "b"); err == nil {
		t.Fatal("expected render error to propagate")
	} else if !strings.Contains(err.Error(), "rendering") {
		t.Errorf("error = %v, want it to mention rendering", err)
	}
}

// weighFor builds a standalone weigh command with a no-op puller and silenced
// cobra output, for exercising argument validation without launching the TUI.
func weighFor(args ...string) *cobra.Command {
	cmd := newWeighCmd(fakePuller{charts: map[string]*chart.Chart{}})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd
}

func TestWeighOCIModeRequiresTwoPositional(t *testing.T) {
	if err := weighFor("only-one").Execute(); err == nil {
		t.Fatal("expected error: OCI mode needs exactly 2 refs")
	}
}

func TestWeighRevisionModeRejectsPositional(t *testing.T) {
	err := weighFor("-r", "revs", "-c", "claim.yaml", "extra-ref").Execute()
	if err == nil {
		t.Fatal("expected error: revision mode takes no positional args")
	}
	if !strings.Contains(err.Error(), "positional") {
		t.Errorf("error should mention positional args, got: %v", err)
	}
}

func TestWeighRevisionModeRequiresBothFlags(t *testing.T) {
	for _, args := range [][]string{{"-r", "revs"}, {"-c", "claim.yaml"}} {
		err := weighFor(args...).Execute()
		if err == nil {
			t.Fatalf("%v: expected error, revision mode requires both -r and -c", args)
		}
		if !strings.Contains(err.Error(), "required") {
			t.Errorf("%v: error should mention both flags are required, got: %v", args, err)
		}
	}
}

// With both flags and no positional args, validation passes and the command
// proceeds to loading — so a bogus claim path yields a claim load error, not an
// arg-shape error and not an unknown-flag error.
func TestWeighRevisionModeAcceptsFlagsThenLoads(t *testing.T) {
	err := weighFor("-r", "/no/such/dir", "-c", "/no/such/claim.yaml").Execute()
	if err == nil {
		t.Fatal("expected a claim load error")
	}
	if !strings.Contains(err.Error(), "claim") {
		t.Errorf("expected a claim load error (passed arg validation), got: %v", err)
	}
}

var errBoom = boomError("boom")

type boomError string

func (e boomError) Error() string { return string(e) }
