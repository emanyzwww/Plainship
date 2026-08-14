package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/style"
)

// newNewCmd 实现 plainship new <路径>.
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliNewUse),
		Short: i18n.T(i18n.CliNewShort),
		Long:  i18n.T(i18n.CliNewLong),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			st := style.For(out)
			s, err := core.CreateSpace(args[0])
			if err != nil {
				return err
			}
			printf(out, "%s", st.Green(i18n.T(i18n.CliNewOk, s.Root)))
			println(out, i18n.T(i18n.CliNewDirs))
			println(out, i18n.T(i18n.CliNewDirDocs))
			println(out, i18n.T(i18n.CliNewDirThemes))
			println(out, i18n.T(i18n.CliNewDirBuild))
			println(out, i18n.T(i18n.CliNewDirConfig))
			println(out, i18n.T(i18n.CliNewDirState))
			println(out, "")
			if s.GitAvailable && s.GitRoot != "" {
				printf(out, "%s", i18n.T(i18n.CliNewGitInit, s.GitRoot))
			} else if s.GitAvailable {
				println(out, i18n.T(i18n.CliNewGitInRepo))
			} else {
				println(out, i18n.T(i18n.CliNewGitMissing))
			}
			println(out, "")
			println(out, i18n.T(i18n.CliNewNext))
			println(out, i18n.T(i18n.CliNewStep1))
			println(out, i18n.T(i18n.CliNewStep2))
			println(out, i18n.T(i18n.CliNewStep3))
			println(out, i18n.T(i18n.CliNewStep4))
			return nil
		},
	}
	return cmd
}

// printf 向指定输出写入格式化内容.
func printf(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, format, args...)
}

// println 向指定输出写入一行内容.
func println(out io.Writer, args ...any) {
	fmt.Fprintln(out, args...)
}
