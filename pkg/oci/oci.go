// Package oci pulls Helm charts from OCI registries.
package oci

import (
	"bytes"
	"fmt"

	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/registry"
)

// Puller pulls a Helm chart from an OCI reference. It is an interface so
// callers can be tested with a fake.
type Puller interface {
	Pull(ref string) (*chart.Chart, error)
}

// RegistryPuller is a Puller backed by the Helm OCI registry client.
type RegistryPuller struct {
	client *registry.Client
}

// NewRegistryPuller constructs a RegistryPuller with a default Helm registry
// client, or returns an error if the client cannot be created.
func NewRegistryPuller() (*RegistryPuller, error) {
	c, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	return &RegistryPuller{client: c}, nil
}

// Pull downloads the chart at ref and loads it from the packaged bytes,
// without writing anything to disk.
func (p *RegistryPuller) Pull(ref string) (*chart.Chart, error) {
	pulled, err := p.client.Pull(ref, registry.PullOptWithChart(true))
	if err != nil {
		return nil, fmt.Errorf("pulling %s: %w", ref, err)
	}

	raw, err := loader.LoadArchive(bytes.NewReader(pulled.Chart.Data))
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", ref, err)
	}

	ch, ok := raw.(*chart.Chart)
	if !ok {
		return nil, fmt.Errorf("%s is not a valid helm chart", ref)
	}

	return ch, nil
}
