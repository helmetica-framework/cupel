package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/revision"
	"github.com/helmetica-framework/cupel/pkg/source"
	"github.com/helmetica-framework/cupel/pkg/tui"
)

// weighSources renders a (before) and b (after) through the source abstraction
// and diffs them with engine. It returns each source's label (for the column
// headers) alongside the diff result. Progress is logged via slog as each
// (potentially slow, network-bound) render runs, before the TUI takes the
// screen.
func weighSources(a, b source.Source, puller oci.Puller, engine diff.Engine) (string, string, diff.Result, error) {
	slog.Info("rendering first chart")

	aMan, aLabel, err := a.Render(puller)
	if err != nil {
		return "", "", diff.Result{}, fmt.Errorf("rendering first chart: %w", err)
	}

	slog.Info("rendering second chart")

	bMan, bLabel, err := b.Render(puller)
	if err != nil {
		return "", "", diff.Result{}, fmt.Errorf("rendering second chart: %w", err)
	}

	res, err := engine.Diff(diff.Rendered{
		Manifest: aMan,
		Ref:      aLabel,
	}, diff.Rendered{
		Manifest: bMan,
		Ref:      bLabel,
	})
	if err != nil {
		return "", "", diff.Result{}, err
	}

	return aLabel, bLabel, res, nil
}

// newWeighCmd builds the weigh command: a static side-by-side diff of two
// sources, each an OCI chart ref ("oci://…") or a cluster claim reference
// ("<kind>/<name>"), auto-detected by source.Parse.
func newWeighCmd(puller oci.Puller) *cobra.Command {
	var engineName, namespace string
	cmd := &cobra.Command{
		Use:   "weigh <A> <B>",
		Short: "Balance one source against another and see which way it tips.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// resolve turns a <kind>/<name> operand into its cluster claim. It
			// has to dial lazily (revision.NewClient inside the closure), so
			// pure oci://-vs-oci:// weighs never touch the cluster, then
			// LoadClaim with cmd.Context() and resolveNamespace(namespace).
			resolve := func(operand string) (revision.Claim, error) {
				c, err := revision.NewClient()
				if err != nil {
					return revision.Claim{}, err
				}

				return c.LoadClaim(cmd.Context(), operand, resolveNamespace(namespace))
			}

			a, err := source.Parse(args[0], resolve)
			if err != nil {
				return err
			}

			b, err := source.Parse(args[1], resolve)
			if err != nil {
				return err
			}

			eng, err := diff.Get(engineName)
			if err != nil {
				return err
			}

			aLabel, bLabel, result, err := weighSources(a, b, puller, eng)
			if err != nil {
				return err
			}

			return tui.Run(aLabel, bLabel, result)
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
	RootCmd.AddCommand(newWeighCmd(puller))
}
