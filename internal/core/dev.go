package core

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/emanyzwww/plainship/internal/builder"
	"github.com/emanyzwww/plainship/internal/dev"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/ui"
)

// DevOptions 控制 Dev 模式行为.
type DevOptions struct {
	Addr string // Addr 是开发服务器监听地址, 默认 :8080.
}

// Dev 启动本地开发模式: 首次构建 -> 启动服务器 -> 监听变更自动重建并热更新.
//
// dev 模式只构建, 不提交 Git, 不打构建编号.
//
// 阻塞运行直到收到 Ctrl+C / SIGINT / SIGTERM.
//
// 输出, 样张 6.3: 标题 → Serving/Watching Detail → 构建状态行, 时间戳由 UI 提供.
func Dev(spaceRoot string, opts DevOptions, out ui.UI) error {
	if out == nil {
		out = ui.Discard
	}
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}
	s, err := space.Load(spaceRoot)
	if err != nil {
		return err
	}
	out.Info(ui.Bold(i18n.T(i18n.DevTitle)))
	out.Info("")
	// Serving/Watching 经 Detail 两列对齐输出, 值含关键值标记.
	listenURL := opts.Addr
	if strings.HasPrefix(listenURL, ":") {
		listenURL = "http://localhost" + listenURL
	}
	out.Detail(i18n.T(i18n.DevServingLabel), ui.Green(ui.Cyan(listenURL)))
	out.Detail(i18n.T(i18n.DevWatchingLabel), "docs/  themes/  plainship.yaml")
	out.Info("")

	// 首次构建, 不提交; dev 构建使用根路径链接, 与 dev 服务器保持一致.
	out.Info(i18n.T(i18n.DevBuilding))
	if _, err := builder.BuildDev(s, out); err != nil {
		return i18n.Errorf(i18n.DevFirstBuildFail, err)
	}
	out.Info("")

	// 开发服务器: 服务 `build/` 并广播热更新事件.
	// 先监听端口, 被占用时立即报错退出.
	srv := dev.NewServer(s.BuildDir())
	addr := opts.Addr
	if !hasPortPrefix(addr) {
		addr = ":" + addr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return i18n.Errorf(i18n.DevListenFail, addr, err)
	}
	go func() {
		if err := http.Serve(ln, srv.Routes()); err != nil && err != http.ErrServerClosed {
			out.Warn(i18n.T(i18n.DevServeFail, err))
		}
	}()

	// 文件监听: 变更 -> 重建 -> 广播 reload.
	// 状态行: 每次重建输出 [HH:MM:SS] 前缀, 由 UI Timestamp 提供, 含成功/失败与耗时.
	roots := []string{
		filepath.Join(s.Root, layout.DocsDir),
		filepath.Join(s.Root, layout.ThemesDir),
		filepath.Join(s.Root, layout.ConfigFile),
	}
	w := dev.NewWatcher(roots, func() {
		start := time.Now()
		out.Info(i18n.T(i18n.DevBuilding))
		res, err := builder.BuildDev(s, out)
		if err != nil {
			// 样张: ✗ Build failed + 错误详情 + Waiting for changes...
			out.Info(ui.Red(i18n.T(i18n.DevBuildFailed)))
			out.Info(err.Error())
			out.Info(ui.Dim(i18n.T(i18n.DevWaiting)))
			return
		}
		out.Success(i18n.T(i18n.DevRebuilt, res.ChangedPages, time.Since(start).Round(100*time.Millisecond).String()))
		srv.Broadcast("reload")
	})
	go w.Start()

	// 等待退出信号.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	w.Stop()
	out.Info(i18n.T(i18n.DevStop))
	return nil
}

// hasPortPrefix 判断地址是否已包含冒号前缀.
func hasPortPrefix(addr string) bool {
	return len(addr) > 0 && addr[0] == ':'
}
