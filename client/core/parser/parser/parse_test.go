package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/model/space"
)

// mkScan 创建临时 Space 并执行扫描, 返回 parse 的输入.
func mkScan(t *testing.T, files map[string]string) *scanner.Result {
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
	res, err := scanner.Scan(&space.Space{Root: root, Layout: space.DefaultLayout()})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return res
}

func findDoc(t *testing.T, res *Result, rel string) (Document, bool) {
	t.Helper()
	for _, d := range res.Docs {
		if d.RelPath == rel {
			return d, true
		}
	}
	return Document{}, false
}

// firstProblemAt 查找 Path 与 Severity 匹配的第一个 Problem; 未找到返回 error.
func firstProblemAt(res *Result, path string, severity scanner.Severity) (scanner.Problem, error) {
	for _, p := range res.Problems {
		if p.Path == path && p.Severity == severity {
			return p, nil
		}
	}
	return scanner.Problem{}, fmt.Errorf("problem not found: %s [%s]", path, severity)
}

// TestParseBasic 锁定标准路径: 带 FM / 无 FM 混合解析.
func TestParseBasic(t *testing.T) {
	scanned := mkScan(t, map[string]string{
		"docs/index.md":          "---\ntitle: 首页\n---\n# Hello\n",
		"docs/guide/intro.md":    "# Intro\n\n无 Front Matter 的文档\n",
		"docs/guide/intro.zh.md": "---\ntitle: 介绍\n---\n# 你好\n",
	})

	res, err := Parse(scanned)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.DocCount() != 3 {
		t.Fatalf("DocCount = %d, want 3", res.DocCount())
	}
	if res.Space == nil {
		t.Error("Space = nil, want 透传")
	}

	idx, ok := findDoc(t, res, "docs/index.md")
	if !ok {
		t.Fatal("docs/index.md 缺失")
	}
	if got := idx.MetaTitle(); got != "首页" {
		t.Errorf("index title = %q, want 首页", got)
	}
	if idx.AST == nil {
		t.Error("index AST = nil")
	}
	if string(idx.Body) != "# Hello\n" {
		t.Errorf("index Body = %q, want %q (FM 已剥离)", idx.Body, "# Hello\n")
	}
	if idx.Hash == "" {
		t.Error("index Hash 为空, want SHA-256")
	}

	intro, ok := findDoc(t, res, "docs/guide/intro.md")
	if !ok {
		t.Fatal("docs/guide/intro.md 缺失")
	}
	if len(intro.Meta) != 0 {
		t.Errorf("intro.Meta = %v, want empty (无 FM)", intro.Meta)
	}

	// 排序与 scanner 一致.
	for i := 1; i < len(res.Docs); i++ {
		if res.Docs[i-1].RelPath > res.Docs[i].RelPath {
			t.Errorf("docs not sorted: %s > %s", res.Docs[i-1].RelPath, res.Docs[i].RelPath)
		}
	}
}

// TestParseProblems 锁定容错: 坏 YAML / 未闭合 FM / 文件缺失均不中断批次.
func TestParseProblems(t *testing.T) {
	scanned := mkScan(t, map[string]string{
		"docs/bad.yaml.md": "---\ntitle: [unclosed\n---\n# Bad\n",
		"docs/unclosed.md": "---\ntitle: x\n",
		"docs/good.md":     "---\ntitle: ok\n---\n# Good\n",
	})
	// 模拟扫描后文件被删除: 追加一条指向不存在文件的 DocEntry.
	scanned.Docs = append(scanned.Docs, scanner.DocEntry{
		RelPath: "docs/missing.md",
		AbsPath: filepath.Join(scanned.Space.Root, "docs", "missing.md"),
	})
	sort.Slice(scanned.Docs, func(i, j int) bool { return scanned.Docs[i].RelPath < scanned.Docs[j].RelPath })

	res, err := Parse(scanned)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// bad.yaml.md: 坏 YAML → error 级 Problem, 文档仍产出 (meta 空).
	if _, err := firstProblemAt(res, "docs/bad.yaml.md", scanner.SeverityError); err != nil {
		t.Errorf("bad.yaml.md: %v", err)
	}
	if bad, ok := findDoc(t, res, "docs/bad.yaml.md"); ok && len(bad.Meta) != 0 {
		t.Errorf("bad.yaml.md Meta = %v, want empty", bad.Meta)
	}

	// unclosed.md: 未闭合 → warning 级 Problem, 正文回退为整篇.
	if _, err := firstProblemAt(res, "docs/unclosed.md", scanner.SeverityWarning); err != nil {
		t.Errorf("unclosed.md: %v", err)
	}
	if u, ok := findDoc(t, res, "docs/unclosed.md"); ok && string(u.Body) != "---\ntitle: x\n" {
		t.Errorf("unclosed.md Body = %q, want 整篇原文回退", u.Body)
	}

	// missing.md: 文件缺失 → error 级 Problem 且文档不入列.
	if _, err := firstProblemAt(res, "docs/missing.md", scanner.SeverityError); err != nil {
		t.Errorf("missing.md: %v", err)
	}
	if _, ok := findDoc(t, res, "docs/missing.md"); ok {
		t.Error("missing.md 不应产出 Document")
	}

	// 好文档不受影响.
	if got := res.DocCount(); got != 3 {
		t.Errorf("DocCount = %d, want 3 (good + bad + unclosed, 不含 missing)", got)
	}
}

// TestParseBOM 锁定 BOM 剥离: 元数据正常, 哈希基于原始内容.
func TestParseBOM(t *testing.T) {
	raw := "\ufeff---\ntitle: bom\n---\n# Hi\n"
	scanned := mkScan(t, map[string]string{"docs/index.md": raw})
	res, err := Parse(scanned)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	idx, ok := findDoc(t, res, "docs/index.md")
	if !ok {
		t.Fatal("docs/index.md 缺失")
	}
	if got := idx.MetaTitle(); got != "bom" {
		t.Errorf("title = %q, want bom (BOM 已剥离)", got)
	}
	if string(idx.Body) != "# Hi\n" {
		t.Errorf("Body = %q, want %q", idx.Body, "# Hi\n")
	}
	if idx.Hash != hashContent([]byte(raw)) {
		t.Error("Hash 应基于原始文件内容 (含 BOM) 计算")
	}
}

// TestParseNilResult 锁定 nil 输入报错.
func TestParseNilResult(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("Parse(nil): expected error, got nil")
	}
}

// TestParseIdempotent 锁定幂等: 同一扫描结果重复解析, 结果一致.
func TestParseIdempotent(t *testing.T) {
	scanned := mkScan(t, map[string]string{
		"docs/index.md":          "---\ntitle: 首页\n---\n# Hello\n",
		"docs/guide/intro.zh.md": "# 介绍\n",
	})
	r1, err := Parse(scanned)
	if err != nil {
		t.Fatalf("Parse #1: %v", err)
	}
	r2, err := Parse(scanned)
	if err != nil {
		t.Fatalf("Parse #2: %v", err)
	}
	if r1.DocCount() != r2.DocCount() {
		t.Errorf("DocCount mismatch: %d vs %d", r1.DocCount(), r2.DocCount())
	}
	for i := range r1.Docs {
		if r1.Docs[i].RelPath != r2.Docs[i].RelPath ||
			r1.Docs[i].Hash != r2.Docs[i].Hash ||
			r1.Docs[i].MetaTitle() != r2.Docs[i].MetaTitle() {
			t.Errorf("docs[%d] mismatch: %+v vs %+v", i, r1.Docs[i], r2.Docs[i])
		}
	}
}
