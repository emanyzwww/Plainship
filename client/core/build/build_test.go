package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emanyzwww/papership-client/model/space"
)

// mkSpace 构造临时 Space.
func mkSpace(t *testing.T, files map[string]string) *space.Space {
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
	return &space.Space{Root: root, Layout: space.DefaultLayout()}
}

// TestRunPipeline 锁定全链路: scan→parse→normalize 后文档数量与语义字段正确,
// 问题被跨阶段汇总并按阶段分组.
func TestRunPipeline(t *testing.T) {
	s := mkSpace(t, map[string]string{
		"papership.yaml":   "site_id: demo\n",
		"docs/index.md":    "---\ntitle: 首页\n---\n# Hello\n",
		"docs/intro.zh.md": "# 介绍\n",
		"docs/bad.yaml.md": "---\ntitle: [unclosed\n---\n# Bad\n",
	})

	res, err := Run(s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DocCount() != 3 {
		t.Fatalf("DocCount = %d, want 3", res.DocCount())
	}

	// 语义字段由 normalizer 写入共享脊柱.
	byPath := map[string]string{}
	for _, d := range res.Docs {
		byPath[d.RelPath] = d.Title + "|" + d.Slug + "|" + d.Lang + "|" + d.Base
	}
	if byPath["docs/index.md"] != "首页|index||index" {
		t.Errorf("index normalized = %q", byPath["docs/index.md"])
	}
	if byPath["docs/intro.zh.md"] != "介绍|intro|zh|intro" {
		t.Errorf("intro.zh normalized = %q", byPath["docs/intro.zh.md"])
	}

	// 问题汇总: 坏 YAML 产生 parser error 问题, 跨阶段合并.
	if res.Summary.Total == 0 {
		t.Error("Summary.Total = 0, want problems aggregated")
	}
	if res.Summary.Errors == 0 {
		t.Errorf("Summary.Errors = 0, want >=1 (bad yaml), got %+v", res.Summary)
	}
	if res.Summary.StageCount != 3 {
		t.Errorf("Summary.StageCount = %d, want 3", res.Summary.StageCount)
	}
	byStage := res.ProblemsByStage()
	if len(byStage["scanner"]) == 0 || len(byStage["parser"]) == 0 {
		t.Errorf("ProblemsByStage missing stages: %+v", byStage)
	}

	// 排序约定.
	for i := 1; i < len(res.Docs); i++ {
		if res.Docs[i-1].RelPath > res.Docs[i].RelPath {
			t.Errorf("docs not sorted: %s > %s", res.Docs[i-1].RelPath, res.Docs[i].RelPath)
		}
	}
}

// TestRunNilSpace 锁定 nil 输入报错.
func TestRunNilSpace(t *testing.T) {
	if _, err := Run(nil); err == nil {
		t.Fatal("Run(nil): expected error, got nil")
	}
}
