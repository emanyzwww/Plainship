package servercli

import (
	"github.com/spf13/cobra"

	"fmt"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/version"
)

// newVersionCmd 实现 plainship-server version.
// 与客户端共享 `internal/version`.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: i18n.T(i18n.CliVersionShort),
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			newUI(cmd).Info(fmt.Sprintf("Plainship Server v%s", version.Version))
		},
	}
}
