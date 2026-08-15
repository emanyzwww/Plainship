package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/ui"
	"github.com/emanyzwww/plainship/internal/version"
)

// newStatusCmd 实现 plainship status.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliStatusUse),
		Short: i18n.T(i18n.CliStatusShort),
		Long:  i18n.T(i18n.CliStatusLong),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			rep, err := core.Status(root)
			if err != nil {
				return err
			}
			// 组件化输出, 样张 6.5: Section 标题 + Detail 两列对齐 + 状态色.
			u.Info(ui.Bold(fmt.Sprintf("Plainship v%s", version.Version)))
			u.Section(i18n.T(i18n.CliStatusSectionSpace))
			u.Detail(i18n.T(i18n.CliStatusRootLabel), rep.SpaceRoot)
			u.Section(i18n.T(i18n.CliStatusSectionGit))
			if rep.GitAvailable && rep.HasRepo {
				u.Detail(i18n.T(i18n.CliStatusBranchLabel), rep.GitBranch)
				if rep.GitClean {
					u.Success(i18n.T(i18n.CliStatusClean))
				} else {
					u.Warn(i18n.T(i18n.CliStatusDirty))
				}
			} else if !rep.GitAvailable {
				u.Info(i18n.T(i18n.CliStatusGitUnavailable))
			} else {
				u.Info(i18n.T(i18n.CliStatusNoRepo))
			}
			u.Section(i18n.T(i18n.CliStatusSectionChanges))
			for _, cat := range revision.Categories {
				c := rep.Changes[cat]
				if c.HasChanges() {
					u.Info(i18n.T(i18n.CliStatusChangeCount, cat, c.Added, c.Modified, c.Deleted))
				} else {
					u.Success(fmt.Sprintf("  %s: clean", cat))
				}
			}
			u.Section(i18n.T(i18n.CliStatusSectionBuild))
			if rep.BuildNumber == "" {
				u.Warn(i18n.T(i18n.CliStatusNotBuilt))
			} else {
				state := ui.Green(i18n.T(i18n.CliStatusLatest))
				if rep.BuildOutdated {
					state = ui.Yellow(i18n.T(i18n.CliStatusOutdated))
				}
				u.Info(i18n.T(i18n.CliStatusBuild, state, ui.Cyan(rep.BuildNumber)))
				if rep.LastBuildTime != "" {
					u.Info(i18n.T(i18n.CliStatusBuildTime, rep.LastBuildTime))
				}
				u.Info(i18n.T(i18n.CliStatusDocCount, rep.DocCount))
			}
			u.Section(i18n.T(i18n.CliStatusSectionPublish))
			if rep.ServerURL == "" {
				u.Warn(i18n.T(i18n.CliStatusServerNone))
			} else {
				u.Info(i18n.T(i18n.CliStatusServer, ui.Cyan(rep.ServerURL)))
				if rep.PublishedBuild == "" {
					u.Warn(i18n.T(i18n.CliStatusPublishedNone))
				} else {
					u.Info(i18n.T(i18n.CliStatusPublished, ui.Cyan(rep.PublishedBuild)))
				}
			}
			u.Flush()
			return nil
		},
	}
	return cmd
}
