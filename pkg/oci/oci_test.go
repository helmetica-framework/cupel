package oci

import "testing"

func TestNewRegistryPullerSatisfiesPuller(t *testing.T) {
	p, err := NewRegistryPuller()
	if err != nil {
		t.Fatalf("NewRegistryPuller: %v", err)
	}
	var _ Puller = p // compile-time interface check
	if p == nil {
		t.Fatal("expected non-nil puller")
	}
}
