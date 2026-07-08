package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/helmetica-framework/cupel/pkg/diff"
	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/render"
	"github.com/helmetica-framework/cupel/pkg/tui"
)

// computeDiff pulls refA and refB using puller, renders both charts, and runs
// the named diff engine. It returns the diff result without launching the TUI.
func computeDiff(puller oci.Puller, engineName, refA, refB string) (diff.Result, error) {
	chA, err := puller.Pull(refA)
	if err != nil {
		return diff.Result{}, err
	}
	chB, err := puller.Pull(refB)
	if err != nil {
		return diff.Result{}, err
	}

	manA, err := render.Render(chA)
	if err != nil {
		return diff.Result{}, fmt.Errorf("rendering %s: %w", refA, err)
	}
	manB, err := render.Render(chB)
	if err != nil {
		return diff.Result{}, fmt.Errorf("rendering %s: %w", refB, err)
	}

	eng, err := diff.Get(engineName)
	if err != nil {
		return diff.Result{}, err
	}

	return eng.Diff(diff.Rendered{Ref: refA, Manifest: manA}, diff.Rendered{Ref: refB, Manifest: manB})
}

// newWeighCmd builds a cobra command that pulls two OCI chart references,
// diffs them, and opens the TUI viewer.
func newWeighCmd(puller oci.Puller) *cobra.Command {
	var engineName string
	cmd := &cobra.Command{
		Use:   "weigh <refA> <refB>",
		Short: "Balance one OCI chart against another and see which way it tips.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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
	return cmd
}

func init() {
	puller, err := oci.NewRegistryPuller()
	cobra.CheckErr(err)
	RootCmd.AddCommand(newWeighCmd(puller))
}
