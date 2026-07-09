package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/render"
	"github.com/helmetica-framework/cupel/pkg/revision"
	"github.com/helmetica-framework/cupel/pkg/tui"
)

// computeDiff pulls refA and refB using puller, renders both charts, and runs
// the named diff engine. It returns the diff result without launching the TUI.
// Progress is logged via slog as each (potentially slow, network-bound) stage
// runs, since this all happens before the TUI takes over the screen.
func computeDiff(puller oci.Puller, engineName, refA, refB string) (diff.Result, error) {
	slog.Info("pulling chart", "ref", refA)
	chA, err := puller.Pull(refA)
	if err != nil {
		return diff.Result{}, err
	}
	slog.Info("pulling chart", "ref", refB)
	chB, err := puller.Pull(refB)
	if err != nil {
		return diff.Result{}, err
	}

	slog.Info("rendering chart", "ref", refA)
	manA, err := render.Render(chA)
	if err != nil {
		return diff.Result{}, fmt.Errorf("rendering %s: %w", refA, err)
	}
	slog.Info("rendering chart", "ref", refB)
	manB, err := render.Render(chB)
	if err != nil {
		return diff.Result{}, fmt.Errorf("rendering %s: %w", refB, err)
	}

	eng, err := diff.Get(engineName)
	if err != nil {
		return diff.Result{}, err
	}

	slog.Info("weighing charts", "engine", engineName)
	return eng.Diff(diff.Rendered{Ref: refA, Manifest: manA}, diff.Rendered{Ref: refB, Manifest: manB})
}

// weighArgs validates the flag/positional combination and returns an error when
// the shape is wrong. It is called at the top of RunE so cobra's Args check is
// bypassed (set to ArbitraryArgs) and we can produce mode-specific messages.
func weighArgs(revDir, claimPath string, positionals []string) error {
	revisionMode := revDir != "" || claimPath != ""
	if revisionMode {
		if revDir == "" || claimPath == "" {
			return fmt.Errorf("both --revisions and --claim are required")
		}
		if len(positionals) > 0 {
			return fmt.Errorf("revision mode takes no positional refs")
		}
		return nil
	}
	// OCI mode: require exactly 2 positional args.
	if len(positionals) != 2 {
		return fmt.Errorf("weigh requires exactly 2 OCI refs, got %d", len(positionals))
	}
	return nil
}

// newWeighCmd builds a cobra command that either diffs two OCI chart references
// (OCI mode) or diffs a directory of InstanceRevision files against a claim
// base (revision mode), opening the appropriate TUI in each case.
func newWeighCmd(puller oci.Puller) *cobra.Command {
	var engineName, revDir, claimPath string
	cmd := &cobra.Command{
		Use:   "weigh <refA> <refB>  |  weigh -r <dir> -c <claim.yaml>",
		Short: "Balance one OCI chart against another and see which way it tips.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := weighArgs(revDir, claimPath, args); err != nil {
				return err
			}

			revisionMode := revDir != "" || claimPath != ""
			if revisionMode {
				slog.Info("loading claim", "path", claimPath)
				claim, err := revision.LoadClaim(claimPath)
				if err != nil {
					return err
				}
				slog.Info("loading revisions", "dir", revDir)
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
			}

			// OCI mode (unchanged).
			result, err := computeDiff(puller, engineName, args[0], args[1])
			if err != nil {
				return err
			}
			return tui.Run(args[0], args[1], result)
		},
	}
	// hidden engine seam; defaults to "linewise", not advertised yet.
	cmd.Flags().StringVar(&engineName, "engine", "linewise", "diff engine")
	_ = cmd.Flags().MarkHidden("engine")
	cmd.Flags().StringVarP(&revDir, "revisions", "r", "", "directory of InstanceRevision YAML files")
	cmd.Flags().StringVarP(&claimPath, "claim", "c", "", "claim YAML file")
	return cmd
}

func init() {
	puller, err := oci.NewRegistryPuller()
	cobra.CheckErr(err)
	RootCmd.AddCommand(newWeighCmd(puller))
}
