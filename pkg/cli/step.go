package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apprenda/kismatic/pkg/install"
	"github.com/apprenda/kismatic/pkg/util"
	"github.com/spf13/cobra"
)

type stepCmd struct {
	out      io.Writer
	planFile string
	task     string
	planner  install.Planner
	executor install.Executor

	// Flags
	generatedAssetsDir string
	restartServices    bool
	verbose            bool
	outputFormat       string
}

// NewCmdStep returns the step command
func NewCmdStep(out io.Writer, installOpts *install.InstallOpts) *cobra.Command {
	stepCmd := &stepCmd{
		out: out,
	}
	cmd := &cobra.Command{
		Use:   "step [CLUSTER_NAME...] PLAY_NAME",
		Short: "run a specific task of the installation workflow (debug feature)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Usage()
			}
			play := args[len(args)-1]

			playExists, err := install.ValidatePlaybookExists(play)
			if !playExists {
				return fmt.Errorf("Playbook %v not found in %v", play, filepath.Join("ansible", "playbooks"))
			}
			if err != nil {
				return err
			}

			var generatedDir, planFile string
			if len(args) == 1 {
				generatedDir, planFile = installOpts.GeneratedDir, installOpts.PlanFile
				if installOpts.GeneratedDir != defaultGeneratedPath && installOpts.PlanFile == defaultClusterPath {
					generatedParent, _ := filepath.Split(installOpts.GeneratedDir)
					planFile = filepath.Join(generatedParent, "kismatic-cluster.yaml")
					generatedDir = installOpts.GeneratedDir
				} else if installOpts.PlanFile != defaultClusterPath && installOpts.GeneratedDir == defaultGeneratedPath {
					planParent, _ := filepath.Split(installOpts.PlanFile)
					generatedDir = filepath.Join(planParent, "generated")
					planFile = installOpts.PlanFile
				}
			}
			if len(args) > 1 {
				clusters := args[0 : len(args)-1]
				if installOpts.GeneratedDir != defaultGeneratedPath || installOpts.PlanFile != defaultClusterPath {
					return fmt.Errorf("cannot specify clusters by name and by generated dir or plan file flags")
				}
				for _, clusterName := range clusters {
					planner := &install.FilePlanner{}
					planner.SetDirs(clusterName)
					stepCmd.planFile = planner.PlanFile
					stepCmd.generatedAssetsDir = planner.GeneratedDir
					execOpts := install.ExecutorOptions{
						GeneratedAssetsDirectory: planner.GeneratedDir,
						OutputFormat:             stepCmd.outputFormat,
						Verbose:                  stepCmd.verbose,
					}
					executor, err := install.NewExecutor(out, os.Stderr, execOpts)
					if err != nil {
						return err
					}

					stepCmd.task = play
					stepCmd.planFile = planner.PlanFile
					stepCmd.planner = planner
					stepCmd.executor = executor
					if err := stepCmd.run(); err != nil {
						return err
					}
				}
				return nil
			}
			stepCmd.planFile = planFile
			stepCmd.generatedAssetsDir = generatedDir
			execOpts := install.ExecutorOptions{
				GeneratedAssetsDirectory: generatedDir,
				OutputFormat:             stepCmd.outputFormat,
				Verbose:                  stepCmd.verbose,
			}
			executor, err := install.NewExecutor(out, os.Stderr, execOpts)
			if err != nil {
				return err
			}

			stepCmd.task = play
			stepCmd.planFile = planFile
			stepCmd.planner = &install.FilePlanner{PlanFile: planFile}
			stepCmd.executor = executor
			return stepCmd.run()

		},
	}
	cmd.Flags().BoolVar(&stepCmd.restartServices, "restart-services", false, "force restart cluster services (Use with care)")
	cmd.Flags().BoolVar(&stepCmd.verbose, "verbose", false, "enable verbose logging from the installation")
	cmd.Flags().StringVarP(&stepCmd.outputFormat, "output", "o", "simple", "installation output format (options \"simple\"|\"raw\")")
	return cmd
}

func (c stepCmd) run() error {
	valOpts := &validateOpts{
		planFile:           c.planFile,
		verbose:            c.verbose,
		outputFormat:       c.outputFormat,
		skipPreFlight:      true,
		generatedAssetsDir: c.generatedAssetsDir,
	}
	if err := doValidate(c.out, c.planner, valOpts); err != nil {
		return err
	}
	plan, err := c.planner.Read()
	if err != nil {
		return fmt.Errorf("error reading plan file: %v", err)
	}
	util.PrintHeader(c.out, "Running Task", '=')
	if err := c.executor.RunPlay(c.task, plan, c.restartServices); err != nil {
		return err
	}
	util.PrintColor(c.out, util.Green, "\nTask completed successfully\n\n")
	return nil
}
