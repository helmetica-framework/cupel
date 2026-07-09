package revision

import (
	"strings"
	"testing"
)

func TestLoadClaim(t *testing.T) {
	c, err := LoadClaim("testdata/claim.yaml")
	if err != nil {
		t.Fatalf("LoadClaim: %v", err)
	}
	if c.OCI != "oci://ghcr.io/stefanprodan/charts/podinfo" {
		t.Errorf("OCI = %q", c.OCI)
	}
	if c.Version != "6.14.0" {
		t.Errorf("Version = %q", c.Version)
	}
	if _, ok := c.Values["replicaCount"]; !ok {
		t.Errorf("Values missing replicaCount: %#v", c.Values)
	}
}

func TestLoadClaimMissingFile(t *testing.T) {
	if _, err := LoadClaim("testdata/does-not-exist.yaml"); err == nil {
		t.Fatal("expected error for missing claim file")
	}
}

func TestLoadRevisionsSortedAscending(t *testing.T) {
	revs, err := LoadRevisions("testdata/revs")
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("got %d revisions, want 2", len(revs))
	}
	if revs[0].Name != "rev-a" || revs[1].Name != "rev-b" {
		t.Errorf("order = [%s, %s], want [rev-a, rev-b] (oldest first)", revs[0].Name, revs[1].Name)
	}
	if !revs[0].Created.Before(revs[1].Created) {
		t.Errorf("Created not ascending: %v then %v", revs[0].Created, revs[1].Created)
	}
}

func TestLoadRevisionsExtractsSpec(t *testing.T) {
	revs, err := LoadRevisions("testdata/revs")
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	b := revs[1] // rev-b
	if b.Version != "6.14.0" {
		t.Errorf("Version = %q, want 6.14.0", b.Version)
	}
	if b.OCI != "oci://ghcr.io/stefanprodan/charts/podinfo" {
		t.Errorf("OCI = %q", b.OCI)
	}
	rc, ok := b.Values["replicaCount"]
	if !ok {
		t.Fatalf("Values missing replicaCount: %#v", b.Values)
	}
	if got := toInt(t, rc); got != 3 {
		t.Errorf("replicaCount = %d, want 3", got)
	}
}

func TestLoadRevisionsAllowsMissingValues(t *testing.T) {
	revs, err := LoadRevisions("testdata/novalues")
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	if revs[0].Values != nil {
		t.Errorf("Values = %#v, want nil for a revision with no spec.values", revs[0].Values)
	}
}

func TestLoadRevisionsFailsOnMalformed(t *testing.T) {
	_, err := LoadRevisions("testdata/bad")
	if err == nil {
		t.Fatal("expected error for malformed revision file")
	}
	if !strings.Contains(err.Error(), "broken.yaml") {
		t.Errorf("error should name the offending file, got: %v", err)
	}
}

// toInt coerces a YAML-decoded number (int/int64/float64) to int for comparison.
func toInt(t *testing.T, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		t.Fatalf("value is %T, want a number", v)
		return 0
	}
}
