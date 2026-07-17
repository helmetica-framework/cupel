package cmd

import "testing"

func TestResolveNamespaceFlagWins(t *testing.T) {
	if got := resolveNamespace("explicit"); got != "explicit" {
		t.Errorf("resolveNamespace(explicit) = %q", got)
	}
}

func TestResolveNamespaceFromKubeconfigContext(t *testing.T) {
	t.Setenv("KUBECONFIG", "testdata/kubeconfig")
	if got := resolveNamespace(""); got != "from-context" {
		t.Errorf("resolveNamespace = %q, want from-context", got)
	}
}

func TestResolveNamespaceFallsBackToDefault(t *testing.T) {
	t.Setenv("KUBECONFIG", "/no/such/kubeconfig")
	if got := resolveNamespace(""); got != "default" {
		t.Errorf("resolveNamespace = %q, want default", got)
	}
}
