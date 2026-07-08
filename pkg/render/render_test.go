package render

import (
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func loadDemo(t *testing.T) *chart.Chart {
	t.Helper()
	raw, err := loader.Load("testdata/demo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ch, ok := raw.(*chart.Chart)
	if !ok {
		t.Fatalf("loaded value is not *chart.Chart")
	}
	return ch
}

func TestRenderProducesManifestWithDefaultValues(t *testing.T) {
	out, err := Render(loadDemo(t))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"kind: Deployment",
		"name: cupel-demo", // fixed dummy release name
		"replicas: 2",
		"image: demo:1.0.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, out)
		}
	}
}
