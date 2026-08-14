package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/version"
)

// newVersionCmd 实现 plainship version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: i18n.T(i18n.CliVersionShort),
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printf(cmd.OutOrStdout(), "Plainship v%s\n", version.Version)
		},
	}
}

// findSpaceRoot 从当前目录向上查找 Space 根目录.
func findSpaceRoot(cmd *cobra.Command) (string, error) {
	return config.FindRoot(".")
}
