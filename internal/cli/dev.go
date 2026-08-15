package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/ui"
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
			// dev 是长驻进程, 输出行带 [HH:MM:SS] 时间戳.
			u := ui.New(ui.Options{
				Out:       cmd.OutOrStdout(),
				Err:       cmd.ErrOrStderr(),
				In:        cmd.InOrStdin(),
				Timestamp: true,
			})
			if flagJSON {
				u = ui.New(ui.Options{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr(), In: cmd.InOrStdin(), Format: ui.FormatJSON})
			}
			return core.Dev(root, core.DevOptions{Addr: addr}, u)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", i18n.T(i18n.CliDevFlagAddr))
	return cmd
}
