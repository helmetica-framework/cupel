package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// ledgerFor builds a standalone ledger command with a no-op puller and silenced
// cobra output, for exercising flag/arg validation without launching the TUI.
func ledgerFor(args ...string) *cobra.Command {
	cmd := newLedgerCmd(fakePuller{charts: map[string]*chart.Chart{}})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd
}

func TestLedgerRequiresClaimFlag(t *testing.T) {
	err := ledgerFor("-r", "somedir").Execute()
	if err == nil {
		t.Fatal("expected error: --claim is required")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention the required flag, got: %v", err)
	}
}

func TestLedgerRequiresRevisionsFlag(t *testing.T) {
	err := ledgerFor("-c", "claim.yaml").Execute()
	if err == nil {
		t.Fatal("expected error: --revisions is required")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention the required flag, got: %v", err)
	}
}

func TestLedgerRejectsPositionalArgs(t *testing.T) {
	err := ledgerFor("-c", "claim.yaml", "-r", "somedir", "extra").Execute()
	if err == nil {
		t.Fatal("expected error: ledger takes no positional args")
	}
}

// With both flags set but a bogus claim path, validation passes and the command
// proceeds to loading — so the error is a claim load error, not a flag error.
func TestLedgerBogusClaimSurfacesLoadError(t *testing.T) {
	err := ledgerFor("-c", "/no/such/claim.yaml", "-r", "/no/such/dir").Execute()
	if err == nil {
		t.Fatal("expected a claim load error")
	}
	if !strings.Contains(err.Error(), "claim") {
		t.Errorf("expected a claim load error (passed flag validation), got: %v", err)
	}
}

// A valid claim but an empty revisions directory hits the empty-set guard.
func TestLedgerEmptyRevisionsGuard(t *testing.T) {
	dir := t.TempDir()
	claimPath := filepath.Join(dir, "claim.yaml")
	if err := os.WriteFile(claimPath, []byte("oci: oci://demo\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	emptyRevs := t.TempDir()

	err := ledgerFor("-c", claimPath, "-r", emptyRevs).Execute()
	if err == nil {
		t.Fatal("expected an empty-revisions error")
	}
	if !strings.Contains(err.Error(), "no revisions") {
		t.Errorf("expected the empty-revisions guard, got: %v", err)
	}
}
