package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/core"
	"github.com/emanyzwww/Plainship/internal/i18n"
)

// newDevCmd 实现 plainship dev.
func newDevCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "dev",
		Short: i18n.T(i18n.CliDevShort),
		Long:  i18n.T(i18n.CliDevLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			return core.Dev(root, core.DevOptions{Addr: addr}, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", i18n.T(i18n.CliDevFlagAddr))
	return cmd
}
