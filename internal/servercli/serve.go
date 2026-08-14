package servercli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/clifx"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/server"
	"github.com/emanyzwww/plainship/internal/style"
)

// newServeCmd 实现 plainship-server serve.
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
					clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeTokenGenerated, tokenFilePath(absData)))
				}
			}
			srv := server.New(dataDir, token)
			st := style.For(out)
			clifx.Printf(out, "%s\n", st.Green(i18n.T(i18n.CliServeStarted)))
			// 展示用监听地址: :9090 -> http://localhost:9090; 其它形式补全 http://.
			listenURL := addr
			if strings.HasPrefix(addr, ":") {
				listenURL = "http://localhost" + addr
			} else if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
				listenURL = "http://" + addr
			}
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeAddr, st.Cyan(listenURL)))
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeDataDir, absData))
			sites := srv.PublishedSites()
			if len(sites) == 0 {
				clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeSitesNone))
			} else {
				clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeSites, strings.Join(sites, ", ")))
			}
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeAuthOn))
			// 醒目打印访问令牌, 供用户复制.
			clifx.Printf(out, "\n%s\n\n", i18n.T(i18n.CliServeTokenBox, st.Cyan(token)))
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeSyncAPI))
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeStatusAPI))
			clifx.Printf(out, "%s\n", i18n.T(i18n.CliServeSiteData))
			clifx.Printf(out, "\n")
			return srv.Serve(addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":9090", i18n.T(i18n.CliServeFlagAddr))
	cmd.Flags().StringVar(&dataDir, "data", "./data", i18n.T(i18n.CliServeFlagData))
	cmd.Flags().StringVar(&token, "token", "", i18n.T(i18n.CliServeFlagToken))
	return cmd
}
