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

func TestRenderWithNilOverlayMatchesRender(t *testing.T) {
	ch := loadDemo(t)
	want, err := Render(ch)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got, err := RenderWith(loadDemo(t), nil)
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if got != want {
		t.Errorf("RenderWith(ch, nil) != Render(ch)\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderWithOverlayOverridesDefaults(t *testing.T) {
	out, err := RenderWith(loadDemo(t), map[string]any{
		"replicas": 5,
		"image":    "demo:9.9.9",
	})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if !strings.Contains(out, "replicas: 5") {
		t.Errorf("overlay replicas not applied:\n%s", out)
	}
	if !strings.Contains(out, "image: demo:9.9.9") {
		t.Errorf("overlay image not applied:\n%s", out)
	}
	// A key the overlay didn't set must keep the chart default.
	if strings.Contains(out, "replicas: 2") {
		t.Errorf("default replicas leaked through overlay:\n%s", out)
	}
}

func TestRenderWithOverlayLeavesUnsetDefaults(t *testing.T) {
	// Overlay only replicas; image must fall back to the chart default.
	out, err := RenderWith(loadDemo(t), map[string]any{"replicas": 7})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if !strings.Contains(out, "replicas: 7") || !strings.Contains(out, "image: demo:1.0.0") {
		t.Errorf("expected replicas:7 + default image:\n%s", out)
	}
}
