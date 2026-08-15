package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/ui"
)

// newNewCmd 实现 plainship new <路径>.
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliNewUse),
		Short: i18n.T(i18n.CliNewShort),
		Long:  i18n.T(i18n.CliNewLong),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			s, err := core.CreateSpace(args[0], u)
			if err != nil {
				return err
			}
			u.Info(strings.TrimSuffix(ui.Green(i18n.T(i18n.CliNewOk, s.Root)), "\n"))
			u.Section(i18n.T(i18n.CliNewSectionDirs))
			u.Info(i18n.T(i18n.CliNewDirDocs))
			u.Info(i18n.T(i18n.CliNewDirThemes))
			u.Info(i18n.T(i18n.CliNewDirBuild))
			u.Info(i18n.T(i18n.CliNewDirConfig))
			u.Info(i18n.T(i18n.CliNewDirState))
			u.Info("")
			if s.GitAvailable && s.GitRoot != "" {
				u.Info(strings.TrimSuffix(i18n.T(i18n.CliNewGitInit, s.GitRoot), "\n"))
			} else if s.GitAvailable {
				u.Info(i18n.T(i18n.CliNewGitInRepo))
			} else {
				u.Info(i18n.T(i18n.CliNewGitMissing))
			}
			u.Section(i18n.T(i18n.CliNewSectionNext))
			u.Info(i18n.T(i18n.CliNewStep1))
			u.Info(i18n.T(i18n.CliNewStep2))
			u.Info(i18n.T(i18n.CliNewStep3))
			u.Info(i18n.T(i18n.CliNewStep4))
			return nil
		},
	}
	return cmd
}
