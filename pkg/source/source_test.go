package source

import (
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmetica-framework/cupel/pkg/revision"
)

// demoChartPath is the shared render fixture, reused so source renders the same
// manifest the render package tests assert on.
const demoChartPath = "../render/testdata/demo"

// fakePuller returns the demo chart for any ref and records the refs it was
// asked to pull, so tests can assert which ref a Source built.
type fakePuller struct {
	ch    *chart.Chart
	pulls []string
}

func newFakePuller(t *testing.T) *fakePuller {
	t.Helper()
	raw, err := loader.Load(demoChartPath)
	if err != nil {
		t.Fatalf("load demo chart: %v", err)
	}
	ch, ok := raw.(*chart.Chart)
	if !ok {
		t.Fatalf("loaded value is not *chart.Chart")
	}
	return &fakePuller{ch: ch}
}

func (p *fakePuller) Pull(ref string) (*chart.Chart, error) {
	p.pulls = append(p.pulls, ref)
	return p.ch, nil
}

func (p *fakePuller) lastPull() string {
	if len(p.pulls) == 0 {
		return ""
	}
	return p.pulls[len(p.pulls)-1]
}

func TestOCIRendersDefaultsLabelIsRef(t *testing.T) {
	p := newFakePuller(t)
	manifest, label, err := OCI("oci://repo/demo:1.0.0").Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if p.lastPull() != "oci://repo/demo:1.0.0" {
		t.Errorf("pulled %q, want the raw ref", p.lastPull())
	}
	if label != "oci://repo/demo:1.0.0" {
		t.Errorf("label = %q, want the ref", label)
	}
	if !strings.Contains(manifest, "replicas: 2") { // chart default, no overlay
		t.Errorf("manifest missing default replicas:\n%s", manifest)
	}
}

func TestClaimAppliesOverlayLabelIsOCIVersion(t *testing.T) {
	p := newFakePuller(t)
	c := revision.Claim{
		OCI:     "oci://repo/demo",
		Version: "2.0.0",
		Values:  map[string]any{"replicas": 5},
	}
	manifest, label, err := Claim(c).Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if p.lastPull() != "oci://repo/demo:2.0.0" {
		t.Errorf("pulled %q, want oci:version", p.lastPull())
	}
	if label != "oci://repo/demo:2.0.0" {
		t.Errorf("label = %q, want oci:version", label)
	}
	if !strings.Contains(manifest, "replicas: 5") {
		t.Errorf("claim overlay not applied:\n%s", manifest)
	}
}

func TestRevisionAppliesOverlayLabelIsName(t *testing.T) {
	p := newFakePuller(t)
	r := revision.Revision{
		Name:    "podinfo-r3",
		OCI:     "oci://repo/demo",
		Version: "3.0.0",
		Values:  map[string]any{"replicas": 7},
	}
	manifest, label, err := Revision(r).Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if p.lastPull() != "oci://repo/demo:3.0.0" {
		t.Errorf("pulled %q, want oci:version", p.lastPull())
	}
	if label != "podinfo-r3" {
		t.Errorf("label = %q, want revision name", label)
	}
	if !strings.Contains(manifest, "replicas: 7") {
		t.Errorf("revision overlay not applied:\n%s", manifest)
	}
}

func TestParseOCIRefIsOCISource(t *testing.T) {
	p := newFakePuller(t)
	// nil resolver: an oci:// operand must never consult it.
	src, err := Parse("oci://repo/demo:1.0.0", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, label, err := src.Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if label != "oci://repo/demo:1.0.0" { // OCI label is the raw ref
		t.Errorf("label = %q, want the ref (OCI source)", label)
	}
}

func TestParseClaimOperandUsesResolver(t *testing.T) {
	resolve := func(operand string) (revision.Claim, error) {
		if operand != "podinfo/my-app" {
			t.Errorf("resolver got %q", operand)
		}
		return revision.Claim{OCI: "oci://repo/demo", Version: "4.0.0", Values: map[string]any{"replicas": 9}}, nil
	}

	src, err := Parse("podinfo/my-app", resolve)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	p := newFakePuller(t)
	manifest, label, err := src.Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if label != "oci://repo/demo:4.0.0" {
		t.Errorf("label = %q, want oci:version (claim source)", label)
	}
	if !strings.Contains(manifest, "replicas: 9") {
		t.Errorf("claim values not applied:\n%s", manifest)
	}
}

func TestParseResolverErrorPropagates(t *testing.T) {
	resolve := func(string) (revision.Claim, error) {
		return revision.Claim{}, errBoomSource
	}
	if _, err := Parse("podinfo/nope", resolve); err == nil {
		t.Fatal("expected resolver error to propagate")
	}
}

var errBoomSource = boomErr("boom")

type boomErr string

func (e boomErr) Error() string { return string(e) }
