package cli

import (
	"io"

	"github.com/apprenda/kismatic/pkg/install"
	"github.com/spf13/cobra"
)

// NewCmdInstall creates a new install command
func NewCmdInstall(in io.Reader, out io.Writer) *cobra.Command {
	opts := &install.InstallOpts{}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "install your Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Subcommands
	cmd.AddCommand(NewCmdPlan(in, out, opts))
	cmd.AddCommand(NewCmdValidate(out, opts))
	cmd.AddCommand(NewCmdApply(out, opts))
	cmd.AddCommand(NewCmdAddNode(out, opts))
	cmd.AddCommand(NewCmdStep(out, opts))
	cmd.AddCommand(NewCmdProvision(in, out, opts))
	cmd.AddCommand(NewCmdDestroy(in, out, opts))
	// PersistentFlags
	addPlanFileFlag(cmd.PersistentFlags(), &opts.PlanFile)
	addGeneratedFlag(cmd.PersistentFlags(), &opts.GeneratedDir)

	return cmd
}
