package build

import (
	"context"
	"testing"

	"github.com/emanyzwww/papership-client/internal/testutil"
)

// TestRunPipeline 锁定全链路: scan→parse→normalize→assemble 后文档数量, 语义字段,
// 图谱投影正确, 问题被跨阶段汇总并按阶段分组.
func TestRunPipeline(t *testing.T) {
	s := testutil.NewSpace(t, map[string]string{
		"papership.yaml":   "site_id: demo\n",
		"docs/index.md":    "---\ntitle: 首页\n---\n# Hello\n",
		"docs/intro.zh.md": "# 介绍\n",
		"docs/bad.yaml.md": "---\ntitle: [unclosed\n---\n# Bad\n",
		"docs/link.md":     "# L\n\n[断链](nope.md)\n",
	})

	res, err := Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DocCount() != 4 {
		t.Fatalf("DocCount = %d, want 4", res.DocCount())
	}

	// 语义字段由 normalizer 写入共享脊柱.
	if d, ok := testutil.Lookup(res.Docs, "docs/index.md"); ok {
		if d.Title != "首页" || d.Slug != "index" || d.Lang != "" {
			t.Errorf("index normalized = Title=%q Slug=%q Lang=%q", d.Title, d.Slug, d.Lang)
		}
	}
	if d, ok := testutil.Lookup(res.Docs, "docs/intro.zh.md"); ok {
		if d.Lang != "zh" || d.Base != "intro" {
			t.Errorf("intro.zh normalized = Lang=%q Base=%q", d.Lang, d.Base)
		}
	}

	// 图谱投影由 assembly 填充: link.md 的断链是顶层文档, 父节点为 docs/index.md.
	if d, ok := testutil.Lookup(res.Docs, "docs/link.md"); ok {
		if d.Parent != "docs/index.md" {
			t.Errorf("link.Parent = %q, want docs/index.md", d.Parent)
		}
	}

	// 问题汇总: 坏 YAML → parser error, 断链 → assembly warning,
	// 缺 themes → scanner warning.
	if res.Summary.Total == 0 || res.Summary.Errors == 0 || res.Summary.Warnings == 0 {
		t.Errorf("Summary = %+v, want total/errors/warnings > 0", res.Summary)
	}
	if res.Summary.StageCount != 4 {
		t.Errorf("Summary.StageCount = %d, want 4", res.Summary.StageCount)
	}
	byStage := res.ProblemsByStage()
	for _, st := range []string{"scanner", "parser", "assembly"} {
		if len(byStage[st]) == 0 {
			t.Errorf("ProblemsByStage missing stage %q: %+v", st, byStage)
		}
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
	if _, err := Run(context.Background(), nil); err == nil {
		t.Fatal("Run(nil): expected error, got nil")
	}
}
