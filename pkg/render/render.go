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

// Render renders the chart to its Kubernetes manifests using the chart's
// default values and a fixed dummy release (cupel/default), equivalent to
// `helm template`. Each template is emitted as a separate YAML document with a
// "# Source" provenance header; NOTES.txt and empty files are skipped.
func Render(ch *chart.Chart) (string, error) {
	vals, err := chartutil.ToRenderValues(ch, ch.Values, common.ReleaseOptions{
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
