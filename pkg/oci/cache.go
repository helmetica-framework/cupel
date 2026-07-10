package oci

import (
	"sync"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// cachingPuller decorates a Puller with an in-memory, concurrency-safe cache
// keyed by OCI ref, so repeated pulls of the same ref hit the cache instead of
// the network. Only successful pulls are cached; errors are never stored, so a
// transient failure can be retried.
type cachingPuller struct {
	inner  Puller
	mu     sync.Mutex
	charts map[string]*chart.Chart
}

// NewCachingPuller wraps inner with an in-memory, concurrency-safe cache keyed
// by OCI ref. It is safe to share the returned Puller across goroutines (e.g.
// the revision TUI's per-revision render commands).
func NewCachingPuller(inner Puller) Puller {
	return &cachingPuller{
		inner:  inner,
		mu:     sync.Mutex{},
		charts: map[string]*chart.Chart{},
	}
}

// Pull returns the cached chart for ref, pulling and caching it on first
// request.
// Pulling happens outside of the lock. So it is possible that
// the same chart is pulled twice, which is a fair trade-off
// since serial pulls can be a lot slower.
func (c *cachingPuller) Pull(ref string) (*chart.Chart, error) {
	c.mu.Lock()

	if chrt, ok := c.charts[ref]; ok {
		c.mu.Unlock()
		return chrt, nil
	}

	c.mu.Unlock()

	// inner already wraps its error with the ref; don't re-wrap.
	chrt, err := c.inner.Pull(ref)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.charts[ref] = chrt
	c.mu.Unlock()

	return chrt, nil
}
