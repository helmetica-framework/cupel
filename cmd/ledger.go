package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/revision"
	"github.com/helmetica-framework/cupel/pkg/tui"
)

// newLedgerCmd builds the ledger command: an interactive browser that diffs a
// claim instance's InstanceRevisions against the claim itself, opening the
// revision TUI. The single operand names the claim
// (<resource>[.<group>]/<name>, resolved in-cluster); -n overrides the
// kubeconfig context namespace.
func newLedgerCmd(puller oci.Puller) *cobra.Command {
	var engineName, namespace string
	cmd := &cobra.Command{
		Use:   "ledger <resource>[.<group>]/<name>",
		Short: "Leaf through a claim's revisions in the cluster.",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			// Mirrors LoadClaim's shape check so anything cobra accepts the
			// client accepts too (k8s names cannot contain "/").
			if parts := strings.Split(args[0], "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("operand %q must be <resource>[.<group>]/<name>, e.g. instance/my-app or instances.podinfo.helmetica-bundles.io/my-app", args[0])
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := revision.NewClient()
			if err != nil {
				return err
			}

			ns := resolveNamespace(namespace)

			claim, err := c.LoadClaim(cmd.Context(), args[0], ns)
			if err != nil {
				return err
			}

			revs, err := c.LoadRevisions(cmd.Context(), ns, claim.UID)
			if err != nil {
				return err
			}

			if len(revs) == 0 {
				return fmt.Errorf("no revisions for claim %s in ns %s", args[0], ns)
			}

			eng, err := diff.Get(engineName)
			if err != nil {
				return err
			}

			approve := func(name string, t time.Time) error {
				return c.Approve(cmd.Context(), ns, name, t)
			}

			return tui.RunRevisions(claim, revs, puller, eng, approve)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace (defaults to the kubeconfig context namespace)")
	// hidden engine seam; defaults to "linewise", not advertised yet.
	cmd.Flags().StringVar(&engineName, "engine", "linewise", "diff engine")
	_ = cmd.Flags().MarkHidden("engine")
	return cmd
}

func init() {
	puller, err := oci.NewRegistryPuller()
	cobra.CheckErr(err)
	RootCmd.AddCommand(newLedgerCmd(puller))
}
