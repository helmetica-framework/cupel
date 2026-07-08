package cmd

import (
	"strings"
	"testing"

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

var errBoom = boomError("boom")

type boomError string

func (e boomError) Error() string { return string(e) }
