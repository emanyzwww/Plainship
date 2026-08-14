// Package git 封装对 Git CLI 的安全调用.
// 所有调用都使用结构化参数, 禁止 shell 拼接, 防止命令注入.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/emanyzwww/Plainship/internal/i18n"
)

// NotFoundError 表示系统未安装 Git.
var NotFoundError = i18n.Errorf(i18n.GitNotFound)

// run 在指定目录执行 git 命令, 返回 stdout.
// 使用 exec.Command 结构化传参, 不经过 shell.
func run(ctx context.Context, dir string, args ...string) (string, string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", "", NotFoundError
	}
	cmd := exec.CommandContext(ctx, gitPath, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

// Available 判断系统是否安装了 Git.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo 判断目录是否位于 Git 仓库内.
// 使用 git rev-parse --is-inside-work-tree.
func IsRepo(dir string) bool {
	out, _, err := run(context.Background(), dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

// Root 返回包含 dir 的 Git 仓库根目录.
// 第二个返回值表示是否找到仓库.
func Root(dir string) (string, bool) {
	out, _, err := run(context.Background(), dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", false
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	return abs, true
}

// Branch 返回当前 Git 分支名.
// 不在仓库中时返回空字符串.
func Branch(dir string) string {
	out, _, err := run(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// 无提交 (unborn HEAD) 时 rev-parse 失败 (exit 128, 输出 "HEAD"),
		// 回退 symbolic-ref 读取分支引用, 例如 refs/heads/master.
		sym, _, serr := run(context.Background(), dir, "symbolic-ref", "HEAD")
		if serr != nil {
			return ""
		}
		return strings.TrimPrefix(strings.TrimSpace(sym), "refs/heads/")
	}
	return strings.TrimSpace(out)
}

// Init 在目录中初始化 Git 仓库.
func Init(dir string) error {
	if IsRepo(dir) {
		return nil
	}
	_, stderr, err := run(context.Background(), dir, "init")
	if err != nil {
		return i18n.Errorf(i18n.GitInitFail, strings.TrimSpace(stderr))
	}
	return nil
}

// StatusEntry 是 git status --porcelain 中的一条记录.
type StatusEntry struct {
	// Status 是两位状态码, 例如 "??" " M" "A " "D " "R ".
	Status string
	// Path 是文件路径(相对仓库根).
	Path string
	// OldPath 用于重命名记录, 其余情况为空.
	OldPath string
}

// Porcelain 执行 git status --porcelain, 返回解析后的条目列表.
func Porcelain(dir string) ([]StatusEntry, error) {
	out, stderr, err := run(context.Background(), dir, "status", "--porcelain", "-z")
	if err != nil {
		if errors.Is(err, NotFoundError) {
			return nil, NotFoundError
		}
		return nil, i18n.Errorf(i18n.GitStatusFail, strings.TrimSpace(stderr))
	}
	return parsePorcelain(out), nil
}

// parsePorcelain 解析 git status --porcelain -z 的输出.
// 使用 NUL 分隔, 兼容中文文件名.
// 格式: "XY PATH\0", 其中 XY 与 PATH 之间有一个空格.
func parsePorcelain(out string) []StatusEntry {
	var entries []StatusEntry
	fields := strings.Split(out, "\x00")
	i := 0
	for i < len(fields) {
		field := fields[i]
		if field == "" {
			i++
			continue
		}
		// 前两位是状态码, 其余是路径(含前导空格).
		if len(field) < 3 {
			i++
			continue
		}
		status := field[:2]
		path := strings.TrimPrefix(field[2:], " ")
		entry := StatusEntry{Status: status, Path: path}
		// 重命名格式: old path 在下一个字段.
		// 状态码 XY 任一位为 R/C 都是重命名/复制 (含工作区未暂存形式 " R").
		if len(status) == 2 && (strings.ContainsRune(status, 'R') || strings.ContainsRune(status, 'C')) {
			if i+1 < len(fields) && fields[i+1] != "" {
				entry.OldPath = fields[i+1]
				i++
			}
		}
		entries = append(entries, entry)
		i++
	}
	return entries
}

// FileChanges 统计指定目录下的文件变化.
// 返回新增、修改、删除的文件路径列表(相对 dir).
// 删除条目无法判断新旧文件名, 这里只报告检测到的状态.
func FileChanges(dir string) (added, modified, deleted []string, err error) {
	entries, err := Porcelain(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, e := range entries {
		code := strings.TrimSpace(e.Status)
		switch {
		case code == "??":
			added = append(added, e.Path)
		case strings.HasPrefix(code, "A"):
			added = append(added, e.Path)
		case strings.HasPrefix(code, "M") || strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C"):
			modified = append(modified, e.Path)
		case strings.HasPrefix(code, "D"):
			deleted = append(deleted, e.Path)
		}
	}
	return added, modified, deleted, nil
}

// Clean 判断工作区是否干净.
func Clean(dir string) bool {
	entries, err := Porcelain(dir)
	if err != nil {
		return false
	}
	return len(entries) == 0
}

// CommitPaths 创建一次提交, 只暂存并提交指定路径.
// paths 相对 cmd.Dir (Space 根目录). 用于分类别提交.
func CommitPaths(dir, message string, paths ...string) error {
	if len(paths) == 0 {
		return i18n.Errorf(i18n.GitCommitNoPaths)
	}
	args := append([]string{"add", "-A", "--"}, paths...)
	if _, _, err := run(context.Background(), dir, args...); err != nil {
		return err
	}
	_, stderr, err := run(context.Background(), dir, "commit", "-m", message)
	if err != nil {
		return i18n.Errorf(i18n.GitCommitFail, strings.TrimSpace(stderr))
	}
	return nil
}

// BuildTagPrefix 是 Plainship 构建编号 tag 的前缀.
const BuildTagPrefix = "ps-"

var buildTagPattern = regexp.MustCompile(`^ps-(\d+)$`)

// NextBuildNumber 计算下一个构建编号.
// 依据现有 ps-* tag 中的最大值 + 1, 格式为 ps-0001.
func NextBuildNumber(dir string) (string, error) {
	out, _, err := run(context.Background(), dir, "tag", "--list", BuildTagPrefix+"*")
	if err != nil {
		return "", err
	}
	max := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := buildTagPattern.FindStringSubmatch(line); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("ps-%04d", max+1), nil
}

// Tag 在 HEAD 创建一个轻量 tag.
// 若同名 tag 已存在 (例如 monorepo 中其他 Space 已占用该编号) 则拒绝创建,
// 避免静默覆盖导致历史丢失.
func Tag(dir, name string) error {
	out, _, err := run(context.Background(), dir, "tag", "--list", name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return i18n.Errorf(i18n.GitTagExists, name)
	}
	_, stderr, err := run(context.Background(), dir, "tag", name)
	if err != nil {
		return i18n.Errorf(i18n.GitTagFail, strings.TrimSpace(stderr))
	}
	return nil
}

// LatestCommitSubject 返回最新一个主题匹配 grepPattern 的提交主题.
// 用于查找某类别 (config/theme/docs) 最新的提交.
func LatestCommitSubject(dir, grepPattern string) (string, bool) {
	out, _, err := run(context.Background(), dir, "log", "-n", "1", "--format=%s", "--grep="+grepPattern)
	if err != nil {
		return "", false
	}
	subject := strings.TrimSpace(out)
	if subject == "" {
		return "", false
	}
	return subject, true
}

// TagsAtHEAD 返回 HEAD 上匹配前缀的所有 tag.
func TagsAtHEAD(dir, prefix string) ([]string, error) {
	out, _, err := run(context.Background(), dir, "tag", "--points-at", "HEAD")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// PassThrough 透传任意 git 命令.
// 参数通过 exec.Command 结构化传递, 不经 shell, 无注入风险.
func PassThrough(dir string, args ...string) (string, string, error) {
	return run(context.Background(), dir, args...)
}

// IsFileTracked 判断文件是否已被 Git 跟踪.
func IsFileTracked(dir, relPath string) bool {
	_, _, err := run(context.Background(), dir, "ls-files", "--error-unmatch", "--", relPath)
	return err == nil
}

// DirExists 判断目录是否存在.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
