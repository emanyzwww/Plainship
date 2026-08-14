package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/config"
	"github.com/emanyzwww/Plainship/internal/core"
	"github.com/emanyzwww/Plainship/internal/fsutil"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/style"
)

// newPublishCmd 实现 plainship publish.
// 交互终端下弹出发布摘要与确认 (--yes 跳过); 非交互环境保持原行为.
func newPublishCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "publish",
		Short: i18n.T(i18n.CliPublishShort),
		Long:  i18n.T(i18n.CliPublishLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			// 二次确认: 交互终端且未 --yes 时弹出摘要; 取消则不发布.
			if !yes && !confirmPublish(cmd, root) {
				return nil
			}
			_, err = core.Publish(root, out)
			return err
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T(i18n.CliPublishFlagYes))
	return cmd
}

// confirmPublish 在交互终端弹出发布摘要与确认.
// 返回 true 表示放行. 仅当 stdin 与 stdout 都指向终端时才交互:
// 任一被重定向 (管道/脚本/CI/测试) 即直接放行, 避免自动化被挂起或误取消.
func confirmPublish(cmd *cobra.Command, root string) bool {
	stdin, ok1 := cmd.InOrStdin().(*os.File)
	stdout, ok2 := cmd.OutOrStdout().(*os.File)
	if !ok1 || !ok2 || !style.IsTerminal(stdin) || !style.IsTerminal(stdout) {
		return true
	}
	// 发布摘要: 站点 / 构建编号 / 文件数 / 目标服务器.
	rep, _ := core.Status(root)
	cfg, _ := config.Load(root)
	buildNum := rep.BuildNumber
	if buildNum == "" {
		buildNum = i18n.T(i18n.CliPreviewUnbuilt)
	}
	files := 0
	if n, err := fsutil.ListFiles(filepath.Join(root, layout.BuildDir)); err == nil {
		files = len(n)
	}
	prompt := i18n.T(i18n.CliPublishConfirm, cfg.Server.Site, buildNum, files, cfg.Server.URL)
	if !askConfirm(stdin, prompt, cmd.OutOrStdout()) {
		st := style.For(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), st.Yellow(i18n.T(i18n.CliPublishCancelled)))
		return false
	}
	return true
}

// askConfirm 输出提示并读取一行, y/yes (大小写不敏感) 返回 true; 其余 (含 EOF) 返回 false.
func askConfirm(in io.Reader, prompt string, out io.Writer) bool {
	fmt.Fprint(out, prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}
