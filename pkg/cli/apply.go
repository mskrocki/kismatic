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

type applyCmd struct {
	out                io.Writer
	planner            install.Planner
	executor           install.Executor
	planFile           string
	generatedAssetsDir string
	verbose            bool
	outputFormat       string
	skipPreFlight      bool
	restartServices    bool
}

type applyOpts struct {
	generatedAssetsDir string
	restartServices    bool
	verbose            bool
	outputFormat       string
	skipPreFlight      bool
}

// NewCmdApply creates a cluter using the plan file
func NewCmdApply(out io.Writer, installOpts *install.InstallOpts) *cobra.Command {
	applyOpts := applyOpts{}
	cmd := &cobra.Command{
		Use:   "apply [CLUSTER_NAME...]",
		Short: "apply your plan file to create a Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			planner := &install.FilePlanner{}
			if installOpts.GeneratedDir == defaultGeneratedPath && installOpts.PlanFile == defaultClusterPath && len(args) > 0 {
				for _, clusterName := range args {
					planner.SetDirs(clusterName)
					executorOpts := install.ExecutorOptions{
						GeneratedAssetsDirectory: planner.PlanFile,
						OutputFormat:             applyOpts.outputFormat,
						Verbose:                  applyOpts.verbose,
					}
					executor, err := install.NewExecutor(out, os.Stderr, executorOpts)
					if err != nil {
						return err
					}

					applyCmd := &applyCmd{
						out:                out,
						planner:            planner,
						executor:           executor,
						planFile:           planner.PlanFile,
						generatedAssetsDir: planner.GeneratedDir,
						verbose:            applyOpts.verbose,
						outputFormat:       applyOpts.outputFormat,
						skipPreFlight:      applyOpts.skipPreFlight,
						restartServices:    applyOpts.restartServices,
					}
					if err := applyCmd.run(); err != nil {
						return err
					}
				}
			} else {
				if len(args) > 0 {
					return fmt.Errorf("Error validating: cannot specify clusters by name and by plan file flag or generated dir flag")
				}
				// Might feel a little strange, but if either generated or plan flags are set, assume the other is in the same place, and not at the default.
				if installOpts.GeneratedDir != defaultGeneratedPath && installOpts.PlanFile != defaultClusterPath {
					planner.PlanFile = installOpts.PlanFile
					planner.GeneratedDir = installOpts.GeneratedDir
				} else if installOpts.GeneratedDir != defaultGeneratedPath {
					generatedParent, _ := filepath.Split(installOpts.GeneratedDir)
					planner.PlanFile = filepath.Join(generatedParent, "kismatic-cluster.yaml")
				} else if installOpts.PlanFile != defaultClusterPath {
					planParent, _ := filepath.Split(installOpts.PlanFile)
					planner.GeneratedDir = filepath.Join(planParent, "generated")
				}
				executorOpts := install.ExecutorOptions{
					GeneratedAssetsDirectory: planner.PlanFile,
					OutputFormat:             applyOpts.outputFormat,
					Verbose:                  applyOpts.verbose,
				}
				executor, err := install.NewExecutor(out, os.Stderr, executorOpts)
				if err != nil {
					return err
				}

				applyCmd := &applyCmd{
					out:                out,
					planner:            planner,
					executor:           executor,
					planFile:           planner.PlanFile,
					generatedAssetsDir: planner.GeneratedDir,
					verbose:            applyOpts.verbose,
					outputFormat:       applyOpts.outputFormat,
					skipPreFlight:      applyOpts.skipPreFlight,
					restartServices:    applyOpts.restartServices,
				}
				if err := applyCmd.run(); err != nil {
					return err
				}
			}
			return nil
		},
	}

	// Flags
	cmd.Flags().BoolVar(&applyOpts.restartServices, "restart-services", false, "force restart cluster services (Use with care)")
	cmd.Flags().BoolVar(&applyOpts.verbose, "verbose", false, "enable verbose logging from the installation")
	cmd.Flags().StringVarP(&applyOpts.outputFormat, "output", "o", "simple", "installation output format (options \"simple\"|\"raw\")")
	cmd.Flags().BoolVar(&applyOpts.skipPreFlight, "skip-preflight", false, "skip pre-flight checks, useful when rerunning kismatic")

	return cmd
}

func (c *applyCmd) run() error {
	// Validate and run pre-flight
	opts := &validateOpts{
		planFile:           c.planFile,
		verbose:            c.verbose,
		outputFormat:       c.outputFormat,
		skipPreFlight:      c.skipPreFlight,
		generatedAssetsDir: c.generatedAssetsDir,
	}
	err := doValidate(c.out, c.planner, opts)
	if err != nil {
		return fmt.Errorf("error validating plan: %v", err)
	}
	plan, err := c.planner.Read()
	if err != nil {
		return fmt.Errorf("error reading plan file: %v", err)
	}

	// Generate certificates
	if err := c.executor.GenerateCertificates(plan, false); err != nil {
		return fmt.Errorf("error installing: %v", err)
	}

	// Generate kubeconfig
	util.PrintHeader(c.out, "Generating Kubeconfig File", '=')
	err = install.GenerateKubeconfig(plan, c.generatedAssetsDir)
	if err != nil {
		return fmt.Errorf("error generating kubeconfig file: %v", err)
	}
	util.PrettyPrintOk(c.out, "Generated kubeconfig file in the %q directory", c.generatedAssetsDir)

	// Perform the installation
	if err := c.executor.Install(plan, c.restartServices); err != nil {
		return fmt.Errorf("error installing: %v", err)
	}

	// Run smoketest
	// Don't run
	if plan.NetworkConfigured() {
		if err := c.executor.RunSmokeTest(plan); err != nil {
			return fmt.Errorf("error running smoke test: %v", err)
		}
	}

	util.PrintColor(c.out, util.Green, "\nThe cluster was installed successfully!\n")
	fmt.Fprintln(c.out)

	msg := "- To use the generated kubeconfig file with kubectl:" +
		"\n    * use \"./kubectl --kubeconfig %s/kubeconfig\"" +
		"\n    * or copy the config file \"cp %[1]s/kubeconfig ~/.kube/config\"\n"
	util.PrintColor(c.out, util.Blue, msg, c.generatedAssetsDir)
	util.PrintColor(c.out, util.Blue, "- To view the Kubernetes dashboard: \"./kismatic dashboard\"\n")
	util.PrintColor(c.out, util.Blue, "- To SSH into a cluster node: \"./kismatic ssh etcd|master|worker|storage|$node.host\"\n")
	fmt.Fprintln(c.out)

	return nil
}
