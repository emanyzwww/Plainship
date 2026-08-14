// token.go 实现 plainship token: 显示服务器访问令牌.
package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/style"
)

// newTokenCmd 实现 plainship token [--data <目录>].
func newTokenCmd() *cobra.Command {
	var dataDir string
	cmd := &cobra.Command{
		Use:   "token",
		Short: i18n.T(i18n.CliTokenShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			absData, err := filepath.Abs(dataDir)
			if err != nil {
				absData = dataDir
			}
			tok, err := LoadToken(dataDir)
			if err != nil {
				if os.IsNotExist(err) {
					return i18n.Errorf(i18n.CliTokenNotFound, tokenFilePath(absData))
				}
				return err
			}
			st := style.For(out)
			printf(out, "%s\n", st.Cyan(i18n.T(i18n.CliTokenValue, tok)))
			printf(out, "%s\n", i18n.T(i18n.CliTokenSavedAt, tokenFilePath(absData)))
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data", "./data", i18n.T(i18n.CliTokenFlagData))
	return cmd
}
