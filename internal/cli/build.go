package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// newBuildCmd 实现 plainship build.
func newBuildCmd() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "build",
		Short: i18n.T(i18n.CliBuildShort),
		Long:  i18n.T(i18n.CliBuildLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			_, err = core.Build(root, core.BuildOptions{Message: message}, cmd.OutOrStdout())
			return err
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", i18n.T(i18n.CliBuildFlagMsg))
	return cmd
}
