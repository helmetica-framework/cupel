package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/revision"
	"github.com/helmetica-framework/cupel/pkg/tui"
)

// newLedgerCmd builds the ledger command: an interactive browser that diffs a
// directory of InstanceRevision files against a claim base, opening the
// revision TUI. Both --claim and --revisions are required; it takes no
// positional args.
func newLedgerCmd(puller oci.Puller) *cobra.Command {
	var engineName, revDir, claimPath string
	cmd := &cobra.Command{
		Use:   "ledger -c <claim.yaml> -r <dir>",
		Short: "Leaf through a release's revisions against its claim.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			claim, err := revision.LoadClaim(claimPath)
			if err != nil {
				return err
			}

			revs, err := revision.LoadRevisions(revDir)
			if err != nil {
				return err
			}

			if len(revs) == 0 {
				return fmt.Errorf("no revisions found in %s", revDir)
			}

			eng, err := diff.Get(engineName)
			if err != nil {
				return err
			}

			return tui.RunRevisions(claim, revs, puller, eng)
		},
	}
	cmd.Flags().StringVarP(&revDir, "revisions", "r", "", "directory of InstanceRevision YAML files")
	cmd.Flags().StringVarP(&claimPath, "claim", "c", "", "claim YAML file")
	_ = cmd.MarkFlagRequired("revisions")
	_ = cmd.MarkFlagRequired("claim")
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
