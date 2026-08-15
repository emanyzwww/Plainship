package servercli

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/server"
	"github.com/emanyzwww/plainship/internal/ui"
	"github.com/emanyzwww/plainship/internal/version"
)

// newServeCmd 实现 plainship-server serve.
func newServeCmd() *cobra.Command {
	var addr, dataDir, token, logLevel, logFile, logFormat string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: i18n.T(i18n.CliServeShort),
		Long:  i18n.T(i18n.CliServeLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			// 运行日志: `--log-level` / `--log-file` / `--log-format` 控制.
			// 默认 info 级别到 stderr, 与用户可见横幅 stdout 分离.
			logger, err := buildLogger(logLevel, logFile, logFormat)
			if err != nil {
				return err
			}
			// 启动时立即创建数据目录.
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return i18n.Errorf(i18n.CliServeMkdirFail, err)
			}
			absData, err := filepath.Abs(dataDir)
			if err != nil {
				absData = dataDir
			}
			// 访问令牌: 显式 `--token` 优先并持久化, 覆盖旧令牌;
			// 未提供时从 `<data>/server.token` 读取, 不存在则自动生成.
			// 认证永远开启, 不存在无认证状态.
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
					u.Info(i18n.T(i18n.CliServeTokenGenerated, tokenFilePath(absData)))
				}
			}
			srv := server.New(dataDir, token).WithLogger(logger)
			// 启动横幅, 样张 6.4: 标题 → Detail 两列对齐 → token 框 → API 列表.
			u.Info(ui.Bold(i18n.T(i18n.CliServeTitle, version.Version)))
			u.Info("")
			// 展示用监听地址: :9090 -> http://localhost:9090; 其它形式补全 http://.
			listenURL := addr
			if strings.HasPrefix(addr, ":") {
				listenURL = "http://localhost" + addr
			} else if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
				listenURL = "http://" + addr
			}
			u.Detail(i18n.T(i18n.CliServeURLLabel), ui.Cyan(listenURL))
			u.Detail(i18n.T(i18n.CliServeDataLabel), absData)
			sites := srv.PublishedSites()
			if len(sites) == 0 {
				u.Detail(i18n.T(i18n.CliServeSitesLabel), "(none)")
			} else {
				u.Detail(i18n.T(i18n.CliServeSitesLabel), strings.Join(sites, ", "))
			}
			u.Detail(i18n.T(i18n.CliServeAuthLabel), ui.Green(i18n.T(i18n.CliServeAuthValue)))
			// 醒目打印访问令牌, 供用户复制.
			u.Info("")
			u.Info(i18n.T(i18n.CliServeTokenBox, ui.Cyan(token)))
			u.Info("")
			// API 列表: 方法青色, 路径原色.
			u.Section(i18n.T(i18n.CliServeAPISection))
			for _, ep := range [][2]string{
				{"POST", "/api/v1/sites/{siteId}/sync"},
				{"GET", "/api/v1/sites/{siteId}/status"},
				{"GET", "/api/v1/sites/{siteId}/releases/{buildId}"},
			} {
				u.Info("  " + ui.Cyan(ep[0]) + "  " + ep[1])
			}
			u.Info("")
			// 访问日志中间件: 记录每个请求的方法/路径/状态码/耗时.
			return srv.ServeHandler(addr, accessLog(srv.Routes(), logger))
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":9090", i18n.T(i18n.CliServeFlagAddr))
	cmd.Flags().StringVar(&dataDir, "data", "./data", i18n.T(i18n.CliServeFlagData))
	cmd.Flags().StringVar(&token, "token", "", i18n.T(i18n.CliServeFlagToken))
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "日志级别 (debug/info/warn/error)")
	cmd.Flags().StringVar(&logFile, "log-file", "", "日志文件路径 (默认 stderr)")
	cmd.Flags().StringVar(&logFormat, "log-format", "text", "日志格式 (text/json)")
	return cmd
}

// buildLogger 按标志构造 slog.Logger: 级别/输出目标/格式.
func buildLogger(level, file, format string) (*slog.Logger, error) {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "info":
		lv = slog.LevelInfo
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		return nil, fmt.Errorf("无效的日志级别 %q (支持 debug/info/warn/error)", level)
	}
	w := os.Stderr
	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("无法打开日志文件: %w", err)
		}
		w = f
	}
	opts := &slog.HandlerOptions{Level: lv}
	var h slog.Handler
	if strings.ToLower(format) == "json" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h), nil
}

// statusRecorder 捕获响应状态码供访问日志使用.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// accessLog 是 HTTP 访问日志中间件: method path status duration.
func accessLog(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}
