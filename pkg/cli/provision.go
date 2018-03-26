package cli

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"github.com/apprenda/kismatic/pkg/install"
	"github.com/apprenda/kismatic/pkg/provision"

	"github.com/spf13/cobra"
)

// NewCmdProvision creates a new provision command
func NewCmdProvision(in io.Reader, out io.Writer, installOpts *install.InstallOpts) *cobra.Command {
	provisionOpts := &provision.ProvisionOpts{}
	cmd := &cobra.Command{
		Use:   "provision [CLUSTER_NAME...]",
		Short: "provision your Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (installOpts.PlanFile != defaultClusterPath || installOpts.GeneratedDir != defaultGeneratedPath) && len(args) > 0 {
				return fmt.Errorf("cannot specify clusters by name and by plan file or generated dir flag")
			}
			planner := &install.FilePlanner{}
			if len(args) == 0 {
				generatedDir, planFile := installOpts.GeneratedDir, installOpts.PlanFile
				if installOpts.GeneratedDir != defaultGeneratedPath && installOpts.PlanFile == defaultClusterPath {
					generatedParent, _ := filepath.Split(installOpts.GeneratedDir)
					planFile = filepath.Join(generatedParent, "kismatic-cluster.yaml")
					generatedDir = installOpts.GeneratedDir
				} else if installOpts.PlanFile != defaultClusterPath && installOpts.GeneratedDir == defaultGeneratedPath {
					planParent, _ := filepath.Split(installOpts.PlanFile)
					generatedDir = filepath.Join(planParent, "generated")
					planFile = installOpts.PlanFile
				}
				planner.PlanFile = planFile
				planner.GeneratedDir = generatedDir
			}
			if len(args) > 0 {
				for _, clusterName := range args {
					planner.SetDirs(clusterName)
					if err := doProvision(out, planner, provisionOpts); err != nil {
						return err
					}
				}
				return nil
			}
			return doProvision(out, planner, provisionOpts)
		},
	}
	cmd.Flags().BoolVar(&provisionOpts.AllowDestruction, "allow-destruction", false, "Allows possible infrastructure destruction through provisioner planning, required if mutation is scaling down (Use with care)")
	return cmd
}

// NewCmdDestroy creates a new destroy command
func NewCmdDestroy(in io.Reader, out io.Writer, installOpts *install.InstallOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [CLUSTER_NAME...]",
		Short: "destroy your provisioned cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (installOpts.PlanFile != defaultClusterPath || installOpts.GeneratedDir != defaultGeneratedPath) && len(args) > 0 {
				return fmt.Errorf("cannot specify clusters by name and by plan file or generated dir flag")
			}
			planner := &install.FilePlanner{}
			if len(args) == 0 {
				generatedDir, planFile := installOpts.GeneratedDir, installOpts.PlanFile
				if installOpts.GeneratedDir != defaultGeneratedPath && installOpts.PlanFile == defaultClusterPath {
					generatedParent, _ := filepath.Split(installOpts.GeneratedDir)
					planFile = filepath.Join(generatedParent, "kismatic-cluster.yaml")
					generatedDir = installOpts.GeneratedDir
				} else if installOpts.PlanFile != defaultClusterPath && installOpts.GeneratedDir == defaultGeneratedPath {
					planParent, _ := filepath.Split(installOpts.PlanFile)
					generatedDir = filepath.Join(planParent, "generated")
					planFile = installOpts.PlanFile
				}
				planner.PlanFile = planFile
				planner.GeneratedDir = generatedDir
			}
			if len(args) > 0 {
				for _, clusterName := range args {
					planner.SetDirs(clusterName)
					if err := doDestroy(out, planner); err != nil {
						return err
					}
				}
				return nil
			}
			return doDestroy(out, planner)
		},
	}
	return cmd
}

type environmentSecretsGetter struct{}

// GetAsEnvironmentVariables returns a slice of the expected environment
// variables sourcing them from the current process' environment.
func (environmentSecretsGetter) GetAsEnvironmentVariables(clusterName string, expected map[string]string) ([]string, error) {
	var vars []string
	var missingVars []string
	for _, expectedEnvVar := range expected {
		val := os.Getenv(expectedEnvVar)
		if val == "" {
			missingVars = append(missingVars, expectedEnvVar)
		}
		vars = append(vars, fmt.Sprintf("%s=%s", expectedEnvVar, val))
	}
	if len(missingVars) > 0 {
		return nil, fmt.Errorf("%v", missingVars)
	}
	return vars, nil
}

func doProvision(out io.Writer, installOpts *install.InstallOpts, provisionOpts *provision.ProvisionOpts) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	user, err := user.Current()
	if err != nil {
		return err
	}
	fp := &install.FilePlanner{PlanFile: installOpts.PlanFile}
	plan, err := fp.Read()
	if err != nil {
		return fmt.Errorf("unable to read plan file: %v", err)
	}
	tf := provision.AnyTerraform{
		ClusterOwner:    user.Username,
		Output:          out,
		BinaryPath:      filepath.Join(path, "terraform"),
		KismaticVersion: install.KismaticVersion.String(),
		ProvidersDir:    filepath.Join(path, "providers"),
		StateDir:        filepath.Join(path, assetsFolder),
		SecretsGetter:   environmentSecretsGetter{},
	}

	updatedPlan, err := tf.Provision(*plan, *provisionOpts)
	if err != nil {
		return err
	}
	if err := fp.Write(updatedPlan); err != nil {
		return fmt.Errorf("error writing updated plan file to %s: %v", installOpts.PlanFile, err)
	}
	return nil
}

func doDestroy(out io.Writer, planner *install.FilePlanner) error {
	plan, err := planner.Read()
	if err != nil {
		return fmt.Errorf("unable to read plan file: %v", err)
	}
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	tf := provision.AnyTerraform{
		Output:          out,
		BinaryPath:      filepath.Join(path, "./terraform"),
		KismaticVersion: install.KismaticVersion.String(),
		ProvidersDir:    filepath.Join(path, "providers"),
		StateDir:        filepath.Join(path, assetsFolder),
		SecretsGetter:   environmentSecretsGetter{},
	}
	return tf.Destroy(plan.Provisioner.Provider, plan.Cluster.Name)
}
