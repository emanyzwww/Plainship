package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/server"
	"github.com/emanyzwww/Plainship/internal/style"
)

// newServeCmd 实现 plainship serve.
func newServeCmd() *cobra.Command {
	var addr, dataDir, token string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: i18n.T(i18n.CliServeShort),
		Long:  i18n.T(i18n.CliServeLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			// 启动时立即创建数据目录, 避免用户疑惑.
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return i18n.Errorf(i18n.CliServeMkdirFail, err)
			}
			absData, err := filepath.Abs(dataDir)
			if err != nil {
				absData = dataDir
			}
			// 访问令牌: 显式 --token 优先并持久化 (覆盖旧令牌);
			// 未提供时从 <data>/server.token 读取, 不存在则自动生成.
			// 认证永远开启, 不存在"无认证"状态.
			if token != "" {
				if err := SaveToken(dataDir, token); err != nil {
					return i18n.Errorf(i18n.CliServeTokenPersistFail, err)
				}
			} else {
				var created bool
				token, created, err = LoadOrCreateToken(dataDir)
				if err != nil {
					return i18n.Errorf(i18n.CliServeTokenLoadFail, err)
				}
				if created {
					printf(out, "%s\n", i18n.T(i18n.CliServeTokenGenerated, tokenFilePath(absData)))
				}
			}
			srv := server.New(dataDir, token)
			st := style.For(out)
			printf(out, "%s\n", st.Green(i18n.T(i18n.CliServeStarted)))
			// 展示用监听地址: :9090 -> http://localhost:9090; 其它形式补全 http://.
			listenURL := addr
			if strings.HasPrefix(addr, ":") {
				listenURL = "http://localhost" + addr
			} else if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
				listenURL = "http://" + addr
			}
			printf(out, "%s\n", i18n.T(i18n.CliServeAddr, st.Cyan(listenURL)))
			printf(out, "%s\n", i18n.T(i18n.CliServeDataDir, absData))
			sites := srv.PublishedSites()
			if len(sites) == 0 {
				printf(out, "%s\n", i18n.T(i18n.CliServeSitesNone))
			} else {
				printf(out, "%s\n", i18n.T(i18n.CliServeSites, strings.Join(sites, ", ")))
			}
			printf(out, "%s\n", i18n.T(i18n.CliServeAuthOn))
			// 醒目打印访问令牌, 供用户复制.
			printf(out, "\n%s\n\n", i18n.T(i18n.CliServeTokenBox, st.Cyan(token)))
			printf(out, "%s\n", i18n.T(i18n.CliServeSyncAPI))
			printf(out, "%s\n", i18n.T(i18n.CliServeStatusAPI))
			printf(out, "%s\n", i18n.T(i18n.CliServeSiteData))
			printf(out, "\n")
			return srv.Serve(addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":9090", i18n.T(i18n.CliServeFlagAddr))
	cmd.Flags().StringVar(&dataDir, "data", "./data", i18n.T(i18n.CliServeFlagData))
	cmd.Flags().StringVar(&token, "token", "", i18n.T(i18n.CliServeFlagToken))
	return cmd
}
