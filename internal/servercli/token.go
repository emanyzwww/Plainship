// token.go 实现 plainship-server token: 显示服务器访问令牌.
package servercli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/ui"
)

// newTokenCmd 实现 plainship-server token [--data <目录>].
func newTokenCmd() *cobra.Command {
	var dataDir string
	cmd := &cobra.Command{
		Use:   "token",
		Short: i18n.T(i18n.CliTokenShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
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
			u.Info(ui.Cyan(i18n.T(i18n.CliTokenValue, tok)))
			u.Info(i18n.T(i18n.CliTokenSavedAt, tokenFilePath(absData)))
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data", "./data", i18n.T(i18n.CliTokenFlagData))
	return cmd
}
