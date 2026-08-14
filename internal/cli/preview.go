package cli

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/builder"
	"github.com/emanyzwww/Plainship/internal/core"
	"github.com/emanyzwww/Plainship/internal/fsutil"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/state"
	"github.com/emanyzwww/Plainship/internal/style"
)

// previewInfo 是 preview 命令的构建摘要.
type previewInfo struct {
	BuildNumber string
	DocCount    int
	Outdated    bool
	DevBuild    bool
}

// previewPlan 检查 build/ 状态并返回预览摘要.
// build/ 不存在时返回错误 (由 suggest 机制给出构建建议).
func previewPlan(root string) (*previewInfo, error) {
	if !fsutil.IsDir(filepath.Join(root, layout.BuildDir)) {
		return nil, i18n.Errorf(i18n.CliPreviewNotBuilt)
	}
	info := &previewInfo{}
	if rep, err := core.Status(root); err == nil {
		info.BuildNumber = rep.BuildNumber
		info.DocCount = rep.DocCount
		info.Outdated = rep.BuildOutdated
	}
	// dev 产物检测: dev 构建使用根路径, 与生产基础路径不一致.
	if s, err := space.Load(root); err == nil {
		if prev, err := state.LoadState(root); err == nil && prev.BasePath != builder.BasePath(s, false) {
			info.DevBuild = true
		}
	}
	return info, nil
}

// previewHandler 返回服务 build 目录的处理器.
func previewHandler(buildDir string) http.Handler {
	return http.FileServer(http.Dir(buildDir))
}

// openBrowser 尝试用系统默认浏览器打开 URL.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// newPreviewCmd 实现 plainship preview: 本地服务 build/ 生产产物, 供发布前验收.
func newPreviewCmd() *cobra.Command {
	var addr string
	var open bool
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliPreviewUse),
		Short: i18n.T(i18n.CliPreviewShort),
		Long:  i18n.T(i18n.CliPreviewLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			st := style.For(out)
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			info, err := previewPlan(root)
			if err != nil {
				return err
			}
			if info.Outdated {
				println(out, st.Yellow(i18n.T(i18n.CliPreviewOutdated)))
			}
			if info.DevBuild {
				println(out, st.Yellow(i18n.T(i18n.CliPreviewDevBuild)))
			}
			// 展示用监听地址: :8080 -> http://localhost:8080; 其它形式补全 http://.
			listenURL := addr
			if strings.HasPrefix(listenURL, ":") {
				listenURL = "http://localhost" + listenURL
			} else if !strings.HasPrefix(listenURL, "http://") && !strings.HasPrefix(listenURL, "https://") {
				listenURL = "http://" + listenURL
			}
			num := info.BuildNumber
			if num == "" {
				num = i18n.T(i18n.CliPreviewUnbuilt)
			}
			println(out, st.Green(i18n.T(i18n.CliPreviewServe, listenURL, num, info.DocCount)))
			if open {
				if err := openBrowser(listenURL); err != nil {
					println(out, st.Yellow(i18n.T(i18n.CliPreviewOpenFail, err)))
				}
			}
			return http.ListenAndServe(addr, previewHandler(filepath.Join(root, layout.BuildDir)))
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", i18n.T(i18n.CliPreviewFlagAddr))
	cmd.Flags().BoolVar(&open, "open", false, i18n.T(i18n.CliPreviewFlagOpen))
	return cmd
}
