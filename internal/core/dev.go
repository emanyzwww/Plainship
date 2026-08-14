// Package core 是 Plainship 的核心编排层.
// 只负责流程编排 (CreateSpace / Build / Publish / Status / Dev),
// Git 语义 (类别划分, 指纹, 提交协议, 编号) 由 internal/revision 提供.
package core

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/emanyzwww/Plainship/internal/builder"
	"github.com/emanyzwww/Plainship/internal/dev"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/version"
)

// DevOptions 控制 Dev 模式行为.
type DevOptions struct {
	// Addr 是开发服务器监听地址, 默认 :8080.
	Addr string
}

// Dev 启动本地开发模式: 首次构建 -> 启动服务器 -> 监听变更自动重建并热更新.
// dev 模式只构建, 不提交 Git, 不打构建编号.
// 阻塞运行直到收到 Ctrl+C / SIGINT / SIGTERM.
func Dev(spaceRoot string, opts DevOptions, out io.Writer) error {
	printl := func(format string, args ...any) {
		if out != nil {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}
	s, err := space.Load(spaceRoot)
	if err != nil {
		return err
	}
	printl("Plainship v%s", version.Version)
	printl("")
	printl(i18n.T(i18n.DevStarted))
	printl(i18n.T(i18n.DevAddr, opts.Addr))
	printl(i18n.T(i18n.DevWatchDirs, s.Root))
	printl("")

	// 首次构建 (不提交). dev 构建使用根路径链接, 与 dev 服务器保持一致.
	printl(i18n.T(i18n.DevFirstBuild))
	if _, err := builder.BuildDev(s, out); err != nil {
		return i18n.Errorf(i18n.DevFirstBuildFail, err)
	}
	printl("")

	// 开发服务器: 服务 build/ 并广播热更新事件.
	// 先监听端口: 端口被占用时立即报错退出, 而不是静默继续运行.
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
			fmt.Fprintln(os.Stderr, i18n.T(i18n.DevServeFail, err))
		}
	}()

	// 文件监听: 变更 -> 重建 -> 广播 reload.
	roots := []string{
		filepath.Join(s.Root, layout.DocsDir),
		filepath.Join(s.Root, layout.ThemesDir),
		filepath.Join(s.Root, layout.ConfigFile),
	}
	w := dev.NewWatcher(roots, func() {
		printl("")
		printl(i18n.T(i18n.DevRebuild))
		res, err := builder.BuildDev(s, out)
		if err != nil {
			printl(i18n.T(i18n.DevRebuildFail, err))
			return
		}
		printl(i18n.T(i18n.DevRebuildOK, res.ChangedPages))
		srv.Broadcast("reload")
	})
	go w.Start()

	printl(i18n.T(i18n.DevReady))

	// 等待退出信号.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	w.Stop()
	printl(i18n.T(i18n.DevStop))
	return nil
}

// hasPortPrefix 判断地址是否已包含端口前缀 (:).
func hasPortPrefix(addr string) bool {
	return len(addr) > 0 && addr[0] == ':'
}
