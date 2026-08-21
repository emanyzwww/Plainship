// Package testutil 提供各层测试共用的脚手架, 消除测试夹具的逐层复制.
//
// 用法约定:
//   - 文件型夹具一律走 NewSpace / NewSpaceWithLayout (临时目录 + 标准/自定义布局);
//   - 按 RelPath 找文档用 Find (缺失即 Fatal) 或 Lookup (非致命);
//   - 找问题用 FindProblem.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/model/space"
)

// NewSpace 创建临时 Space: 按 files (相对 Space 根的路径 → 内容) 写入文件, 使用标准布局.
func NewSpace(t *testing.T, files map[string]string) *space.Space {
	t.Helper()
	return NewSpaceWithLayout(t, space.DefaultLayout(), files)
}

// NewSpaceWithLayout 创建临时 Space, 使用指定布局.
func NewSpaceWithLayout(t *testing.T, layout space.Layout, files map[string]string) *space.Space {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &space.Space{Root: root, Layout: layout}
}

// Find 在按 RelPath 键的文档列表中查找目标; 缺失即 Fatal.
//
// 传入的文档类型只需实现 pipeline.Keyed (内嵌 pipeline.Doc 或自带 Key()).
func Find[T pipeline.Keyed](t *testing.T, docs []T, rel string) T {
	t.Helper()
	for _, d := range docs {
		if d.Key() == rel {
			return d
		}
	}
	t.Fatalf("文档 %q 缺失", rel)
	var zero T
	return zero
}

// Lookup 在按 RelPath 键的文档列表中查找目标 (非致命).
func Lookup[T pipeline.Keyed](docs []T, rel string) (T, bool) {
	for _, d := range docs {
		if d.Key() == rel {
			return d, true
		}
	}
	var zero T
	return zero, false
}

// FindProblem 在问题列表中查找指定路径与严重级别的问题; 未找到 ok 为 false.
func FindProblem(problems []pipeline.Problem, path string, severity pipeline.Severity) (pipeline.Problem, bool) {
	for _, p := range problems {
		if p.Path == path && p.Severity == severity {
			return p, true
		}
	}
	return pipeline.Problem{}, false
}

// HasProblem 报告问题列表中是否存在指定路径的问题 (不限定严重级别).
func HasProblem(problems []pipeline.Problem, path string) bool {
	for _, p := range problems {
		if p.Path == path {
			return true
		}
	}
	return false
}
