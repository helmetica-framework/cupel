// Package render renders a Helm chart to its Kubernetes manifests using
// default values and a fixed dummy release, equivalent to `helm template`.
package render

import (
	"fmt"
	"sort"
	"strings"

	"helm.sh/helm/v4/pkg/chart/common"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/engine"
)

// RenderWith renders the chart to its Kubernetes manifests using a fixed dummy
// release (cupel/default), equivalent to `helm template`. Values from overlay
// take precedence over the chart's defaults; keys the overlay omits fall back
// to the chart values. A nil overlay renders with the defaults alone. Each
// template is emitted as a separate YAML document with a "# Source" provenance
// header; NOTES.txt and empty files are skipped.
func RenderWith(ch *chart.Chart, overlay map[string]any) (string, error) {
	merged := ch.Values

	if overlay != nil {
		// CoalesceTables treats its first argument as authoritative and merges
		// the chart defaults in beneath it, so overlay wins.
		// This mutates the caller's overlay map (defaults get folded
		// in). Harmless here since both diff sides fold in the same defaults;
		// clone the overlay first if a caller ever needs it left untouched.
		merged = chartutil.CoalesceTables(overlay, merged)
	}

	vals, err := chartutil.ToRenderValues(ch, merged, common.ReleaseOptions{
		Name:      "cupel",
		Namespace: "default",
	}, nil)
	if err != nil {
		return "", fmt.Errorf("merge values: %w", err)
	}

	files, err := engine.Render(ch, vals)
	if err != nil {
		return "", fmt.Errorf("rendering chart: %w", err)
	}

	fileNames := make([]string, 0, len(files))
	for k := range files {
		fileNames = append(fileNames, k)
	}
	sort.Strings(fileNames)

	var b strings.Builder

	for _, k := range fileNames {
		if strings.HasSuffix(k, "NOTES.txt") {
			continue
		}

		content := strings.TrimSpace(files[k])
		if content == "" {
			continue
		}

		fmt.Fprintf(&b, "---\n# Source: %s\n%s\n", k, content)
	}

	return b.String(), nil
}

// Render renders the chart with its default values, equivalent to
// RenderWith(ch, nil).
func Render(ch *chart.Chart) (string, error) {
	return RenderWith(ch, nil)
}
