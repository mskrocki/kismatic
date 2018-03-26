package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/pflag"
)

var defaultClusterPath = filepath.Join("clusters", "kubernetes", "kismatic-kismatic.yaml")
var defaultGeneratedPath = filepath.Join("clusters", "kubernetes", "generated")

func addPlanFileFlag(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "plan-file", "f", defaultClusterPath, "path to the installation plan file")
}

func addGeneratedFlag(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "generated-assets-dir", "g", defaultGeneratedPath, "path to the directory where assets generated during the installation process will be stored")
}

type planFileNotFoundErr struct {
	filename string
}

func (e planFileNotFoundErr) Error() string {
	return fmt.Sprintf("Plan file not found at %q. If you don't have a plan file, you may generate one with 'kismatic install plan'", e.filename)
}
