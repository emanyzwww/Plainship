package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/style"
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
			out := cmd.OutOrStdout()
			st := style.For(out)
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			rep, err := core.Status(root)
			if err != nil {
				return err
			}
			printf(out, "%s\n\n", st.Bold(fmt.Sprintf("Plainship v%s", version.Version)))
			println(out, st.Bold(i18n.T(i18n.CliStatusSpace)))
			printf(out, "  %s\n\n", rep.SpaceRoot)
			println(out, st.Bold(i18n.T(i18n.CliStatusGit)))
			if rep.GitAvailable && rep.HasRepo {
				printf(out, "%s\n", i18n.T(i18n.CliStatusBranch, rep.GitBranch))
				if rep.GitClean {
					println(out, st.Green(i18n.T(i18n.CliStatusClean)))
				} else {
					println(out, st.Yellow(i18n.T(i18n.CliStatusDirty)))
				}
			} else if !rep.GitAvailable {
				println(out, i18n.T(i18n.CliStatusGitUnavailable))
			} else {
				println(out, i18n.T(i18n.CliStatusNoRepo))
			}
			println(out, "")
			println(out, st.Bold(i18n.T(i18n.CliStatusChanges)))
			for _, cat := range revision.Categories {
				c := rep.Changes[cat]
				if c.HasChanges() {
					printf(out, "%s\n", i18n.T(i18n.CliStatusChangeCount, cat, c.Added, c.Modified, c.Deleted))
				} else {
					printf(out, "%s\n", st.Green(fmt.Sprintf("  %s: clean", cat)))
				}
			}
			println(out, "")
			println(out, st.Bold(i18n.T(i18n.CliStatusBrand)))
			if rep.BuildNumber == "" {
				println(out, st.Yellow(i18n.T(i18n.CliStatusNotBuilt)))
			} else {
				state := st.Green(i18n.T(i18n.CliStatusLatest))
				if rep.BuildOutdated {
					state = st.Yellow(i18n.T(i18n.CliStatusOutdated))
				}
				printf(out, "%s\n", i18n.T(i18n.CliStatusBuild, state, st.Cyan(rep.BuildNumber)))
				if rep.LastBuildTime != "" {
					printf(out, "%s\n", i18n.T(i18n.CliStatusBuildTime, rep.LastBuildTime))
				}
				printf(out, "%s\n", i18n.T(i18n.CliStatusDocCount, rep.DocCount))
			}
			println(out, "")
			println(out, st.Bold(i18n.T(i18n.CliStatusPublish)))
			if rep.ServerURL == "" {
				println(out, st.Yellow(i18n.T(i18n.CliStatusServerNone)))
			} else {
				printf(out, "%s\n", i18n.T(i18n.CliStatusServer, st.Cyan(rep.ServerURL)))
				if rep.PublishedBuild == "" {
					println(out, st.Yellow(i18n.T(i18n.CliStatusPublishedNone)))
				} else {
					printf(out, "%s\n", i18n.T(i18n.CliStatusPublished, st.Cyan(rep.PublishedBuild)))
				}
			}
			return nil
		},
	}
	return cmd
}
