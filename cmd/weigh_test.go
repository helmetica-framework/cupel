package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/source"
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

func linewise(t *testing.T) diff.Engine {
	t.Helper()
	eng, err := diff.Get("linewise")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	return eng
}

// Two identical OCI sources diff to no changes, and weighSources returns each
// source's label (the ref) in order.
func TestWeighSourcesIdenticalHaveNoChanges(t *testing.T) {
	ch := loadChart(t, "../pkg/render/testdata/demo")
	p := fakePuller{charts: map[string]*chart.Chart{"oci://a": ch, "oci://b": ch}}

	la, lb, res, err := weighSources(source.OCI("oci://a"), source.OCI("oci://b"), p, linewise(t))
	if err != nil {
		t.Fatalf("weighSources: %v", err)
	}
	if la != "oci://a" || lb != "oci://b" {
		t.Errorf("labels = (%q,%q), want (oci://a, oci://b)", la, lb)
	}
	added, removed := res.Counts()
	if added != 0 || removed != 0 {
		t.Errorf("counts = (%d,%d), want (0,0)", added, removed)
	}
	if len(res.Rows) == 0 {
		t.Error("expected rows for a non-empty render")
	}
}

func TestWeighSourcesPropagatesPullError(t *testing.T) {
	p := fakePuller{err: errBoom}
	if _, _, _, err := weighSources(source.OCI("oci://a"), source.OCI("oci://b"), p, linewise(t)); err == nil {
		t.Fatal("expected pull error to propagate")
	} else if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want it to mention boom", err)
	}
}

func TestWeighSourcesPropagatesRenderError(t *testing.T) {
	// A chart whose template calls the `fail` function cannot render.
	bad := &chart.Chart{
		Metadata:  &chart.Metadata{APIVersion: "v2", Name: "bad", Version: "0.1.0"},
		Templates: []*common.File{{Name: "templates/x.yaml", Data: []byte(`{{ fail "boom" }}`)}},
	}
	p := fakePuller{charts: map[string]*chart.Chart{"oci://a": bad, "oci://b": bad}}
	if _, _, _, err := weighSources(source.OCI("oci://a"), source.OCI("oci://b"), p, linewise(t)); err == nil {
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

func TestWeighRequiresExactlyTwoArgs(t *testing.T) {
	for _, args := range [][]string{{}, {"oci://a"}, {"oci://a", "oci://b", "oci://c"}} {
		if err := weighFor(args...).Execute(); err == nil {
			t.Errorf("%v: expected ExactArgs(2) error", args)
		}
	}
}

// A non-oci operand is a cluster claim reference; without a reachable cluster
// the resolver's client construction error surfaces before the TUI opens.
func TestWeighClaimOperandWithoutClusterErrors(t *testing.T) {
	t.Setenv("KUBECONFIG", "/no/such/kubeconfig")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	err := weighFor("oci://a", "podinfo/my-app").Execute()
	if err == nil {
		t.Fatal("expected an error for a claim operand without a cluster")
	}
}

var errBoom = boomError("boom")

type boomError string

func (e boomError) Error() string { return string(e) }
