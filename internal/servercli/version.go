package servercli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/clifx"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/version"
)

// newVersionCmd 实现 plainship-server version.
// 与客户端共享 internal/version, 保证两个二进制版本一一对应.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: i18n.T(i18n.CliVersionShort),
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			clifx.Printf(cmd.OutOrStdout(), "Plainship Server v%s\n", version.Version)
		},
	}
}
