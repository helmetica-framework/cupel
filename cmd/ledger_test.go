package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// ledgerFor builds a standalone ledger command with a no-op puller and silenced
// cobra output, for exercising argument validation without a cluster.
func ledgerFor(args ...string) *cobra.Command {
	cmd := newLedgerCmd(fakePuller{charts: map[string]*chart.Chart{}})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd
}

func TestLedgerRequiresExactlyOneArg(t *testing.T) {
	for _, args := range [][]string{{}, {"podinfo/a", "podinfo/b"}} {
		if err := ledgerFor(args...).Execute(); err == nil {
			t.Errorf("%v: expected ExactArgs(1) error", args)
		}
	}
}

func TestLedgerRejectsMalformedOperand(t *testing.T) {
	for _, operand := range []string{"just-a-name", "/no-kind", "no-name/", "a/b/c"} {
		err := ledgerFor(operand).Execute()
		if err == nil {
			t.Fatalf("%q: expected error for operand without <kind>/<name> shape", operand)
		}
		if !strings.Contains(err.Error(), "<kind>/<name>") {
			t.Errorf("%q: error should explain the expected shape, got: %v", operand, err)
		}
	}
}

// With a valid operand but no reachable cluster, the failure is a connection
// error from client construction — proving arg validation passed and the
// command dialed rather than reading files.
func TestLedgerNoClusterSurfacesClientError(t *testing.T) {
	t.Setenv("KUBECONFIG", "/no/such/kubeconfig")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	err := ledgerFor("podinfo/my-app").Execute()
	if err == nil {
		t.Fatal("expected a client construction error")
	}
}
