package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"os"

	"github.com/apprenda/kismatic/pkg/install"
	"github.com/apprenda/kismatic/pkg/util"
	"github.com/spf13/cobra"
)

type validateOpts struct {
	generatedAssetsDir string
	planFile           string
	verbose            bool
	outputFormat       string
	skipPreFlight      bool
}

// NewCmdValidate creates a new install validate command
func NewCmdValidate(out io.Writer, installOpts *install.InstallOpts) *cobra.Command {
	opts := &validateOpts{}
	cmd := &cobra.Command{
		Use:   "validate CLUSTER_NAME",
		Short: "validate your plan file",
		RunE: func(cmd *cobra.Command, args []string) error {
			planner := &install.FilePlanner{}
			if installOpts.GeneratedDir == defaultGeneratedPath && installOpts.PlanFile == defaultClusterPath && len(args) > 0 {
				for _, clusterName := range args {
					planner.SetDirs(clusterName)
					if err := doValidate(out, planner, opts); err != nil {
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
				if err := doValidate(out, planner, opts); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "enable verbose logging from the installation")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "o", "simple", "installation output format (options simple|raw)")
	cmd.Flags().BoolVar(&opts.skipPreFlight, "skip-preflight", false, "skip pre-flight checks")
	return cmd
}

func doValidate(out io.Writer, planner install.Planner, opts *validateOpts) error {
	util.PrintHeader(out, "Validating", '=')
	// Check if plan file exists
	if !planner.PlanExists() {
		util.PrettyPrintErr(out, "Reading installation plan file [ERROR]")
		fmt.Fprintln(out, "Run \"kismatic install plan\" to generate it")
		return fmt.Errorf("plan does not exist")
	}
	plan, err := planner.Read()
	if err != nil {
		util.PrettyPrintErr(out, "Reading installation plan file %q", opts.planFile)
		return fmt.Errorf("error reading plan file: %v", err)
	}
	util.PrettyPrintOk(out, "Reading installation plan file %q", opts.planFile)

	// Validate plan file
	if err := validatePlan(out, plan); err != nil {
		return err
	}

	// Validate SSH connections
	if err := validateSSHConnectivity(out, plan); err != nil {
		return err
	}

	// get a new pki
	pki, err := newPKI(out, opts)
	if err != nil {
		return err
	}
	// Validate Certificates
	ok, errs := install.ValidateCertificates(plan, pki)
	if !ok {
		util.PrettyPrintErr(out, "Validating cluster certificates")
		util.PrintValidationErrors(out, errs)
		return fmt.Errorf("Cluster certificates validation error prevents installation from proceeding")
	}

	if opts.skipPreFlight {
		return nil
	}
	// Run pre-flight
	options := install.ExecutorOptions{
		OutputFormat: opts.outputFormat,
		Verbose:      opts.verbose,
	}
	e, err := install.NewPreFlightExecutor(out, os.Stderr, options)
	if err != nil {
		return err
	}
	return e.RunPreFlightCheck(plan)
}

// TODO this should really not be here
func newPKI(stdout io.Writer, options *validateOpts) (*install.LocalPKI, error) {
	ansibleDir := "ansible"
	if options.generatedAssetsDir == "" {
		return nil, fmt.Errorf("GeneratedAssetsDirectory option cannot be empty")
	}
	certsDir := filepath.Join(options.generatedAssetsDir, "keys")
	pki := &install.LocalPKI{
		CACsr: filepath.Join(ansibleDir, "playbooks", "tls", "ca-csr.json"),
		GeneratedCertsDirectory: certsDir,
		Log: stdout,
	}
	return pki, nil
}

func validatePlan(out io.Writer, plan *install.Plan) error {
	ok, errs := install.ValidatePlan(plan)
	if !ok {
		util.PrettyPrintErr(out, "Validating installation plan file")
		util.PrintValidationErrors(out, errs)
		return fmt.Errorf("Plan file validation error prevents installation from proceeding")
	}
	util.PrettyPrintOk(out, "Validating installation plan file")
	return nil
}

func validateSSHConnectivity(out io.Writer, plan *install.Plan) error {
	ok, errs := install.ValidatePlanSSHConnections(plan)
	if !ok {
		util.PrettyPrintErr(out, "Validating SSH connectivity to nodes")
		util.PrintValidationErrors(out, errs)
		return fmt.Errorf("SSH connectivity validation error prevents installation from proceeding")
	}
	util.PrettyPrintOk(out, "Validating SSH connectivity to nodes")
	return nil
}
