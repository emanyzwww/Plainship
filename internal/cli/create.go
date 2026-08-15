package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/ui"
)

// newCreateCmd 实现 plainship create <名称>.
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliCreateUse),
		Short: i18n.T(i18n.CliCreateShort),
		Long:  i18n.T(i18n.CliCreateLong),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			rel, err := core.CreateDocument(root, args[0])
			if err != nil {
				return err
			}
			u.Info(ui.Green(i18n.T(i18n.CliCreateOk, rel)))
			u.Info("")
			u.Info(i18n.T(i18n.CliCreateEdit, rel))
			return nil
		},
	}
	return cmd
}
