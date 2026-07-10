package oci

import (
	"errors"
	"sync"
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// countingPuller is a fake Puller that records how many times each ref was
// pulled and optionally fails a configurable number of leading calls.
type countingPuller struct {
	mu    sync.Mutex
	calls map[string]int
	// failFirst[ref] is the number of leading Pull calls for ref that return
	// an error before the puller starts succeeding.
	failFirst map[string]int
}

func newCountingPuller() *countingPuller {
	return &countingPuller{calls: map[string]int{}, failFirst: map[string]int{}}
}

func (p *countingPuller) Pull(ref string) (*chart.Chart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[ref]++
	if p.calls[ref] <= p.failFirst[ref] {
		return nil, errors.New("boom")
	}
	return &chart.Chart{Metadata: &chart.Metadata{Name: ref}}, nil
}

func (p *countingPuller) count(ref string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[ref]
}

func TestCachingPullerCachesRepeatedRef(t *testing.T) {
	inner := newCountingPuller()
	c := NewCachingPuller(inner)

	first, err := c.Pull("oci://repo/chart:1.0.0")
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}
	second, err := c.Pull("oci://repo/chart:1.0.0")
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	if got := inner.count("oci://repo/chart:1.0.0"); got != 1 {
		t.Fatalf("inner pulled %d times, want 1 (cache miss on second call)", got)
	}
	if first != second {
		t.Fatal("cached Pull returned a different chart pointer than the first")
	}
}

func TestCachingPullerDistinctRefsPullSeparately(t *testing.T) {
	inner := newCountingPuller()
	c := NewCachingPuller(inner)

	if _, err := c.Pull("oci://repo/a:1"); err != nil {
		t.Fatalf("Pull a: %v", err)
	}
	if _, err := c.Pull("oci://repo/b:1"); err != nil {
		t.Fatalf("Pull b: %v", err)
	}

	if got := inner.count("oci://repo/a:1"); got != 1 {
		t.Fatalf("ref a pulled %d times, want 1", got)
	}
	if got := inner.count("oci://repo/b:1"); got != 1 {
		t.Fatalf("ref b pulled %d times, want 1", got)
	}
}

func TestCachingPullerDoesNotCacheErrors(t *testing.T) {
	inner := newCountingPuller()
	inner.failFirst["oci://repo/flaky:1"] = 1 // first call errors, then succeeds
	c := NewCachingPuller(inner)

	if _, err := c.Pull("oci://repo/flaky:1"); err == nil {
		t.Fatal("expected first Pull to error")
	}
	if _, err := c.Pull("oci://repo/flaky:1"); err != nil {
		t.Fatalf("expected second Pull to succeed after retry, got %v", err)
	}

	if got := inner.count("oci://repo/flaky:1"); got != 2 {
		t.Fatalf("inner pulled %d times, want 2 (error must not be cached)", got)
	}
}
