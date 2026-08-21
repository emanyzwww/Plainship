package normalizer

import (
	"context"
	"testing"

	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/internal/testutil"
)

// normalizeFixture 构造临时 Space, 走完 scan → parse → normalize 全链路.
func normalizeFixture(t *testing.T, files map[string]string) *Result {
	t.Helper()
	ctx := context.Background()
	sp := testutil.NewSpace(t, files)
	scanned, err := scanner.Scan(ctx, sp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	parsed, err := parser.Parse(ctx, scanned)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Normalize(ctx, parsed)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return res
}

// TestNormalizeLanguageSuffix 锁定语言后缀: 提取/忽略/带地区/全大写不误判.
func TestNormalizeLanguageSuffix(t *testing.T) {
	res := normalizeFixture(t, map[string]string{
		"docs/intro.zh.md":    "# 介绍\n",
		"docs/guide.en-us.md": "# Guide\n",
		"docs/case.EN.md":     "# Case\n",
		"docs/plain.md":       "# Plain\n",
	})
	if res.DocCount() != 4 {
		t.Fatalf("DocCount = %d, want 4", res.DocCount())
	}

	// intro.zh → 拆出语言, 基名去后缀.
	if d, ok := testutil.Lookup(res.Docs, "docs/intro.zh.md"); ok {
		if d.Lang != "zh" || d.Base != "intro" {
			t.Errorf("intro.zh: Lang=%q Base=%q, want zh/intro", d.Lang, d.Base)
		}
	} else {
		t.Error("docs/intro.zh.md 缺失")
	}

	// en-us → 带地区后缀的语言码.
	if d, ok := testutil.Lookup(res.Docs, "docs/guide.en-us.md"); ok {
		if d.Lang != "en-us" || d.Base != "guide" {
			t.Errorf("guide.en-us: Lang=%q Base=%q, want en-us/guide", d.Lang, d.Base)
		}
	} else {
		t.Error("docs/guide.en-us.md 缺失")
	}

	// case.EN → 全大写不识别为语言后缀 (基名原样保留).
	if d, ok := testutil.Lookup(res.Docs, "docs/case.EN.md"); ok {
		if d.Lang != "" || d.Base != "case.EN" {
			t.Errorf("case.EN: Lang=%q Base=%q, want ''/case.EN", d.Lang, d.Base)
		}
	} else {
		t.Error("docs/case.EN.md 缺失")
	}

	// plain → 无后缀.
	if d, ok := testutil.Lookup(res.Docs, "docs/plain.md"); ok && d.Lang != "" {
		t.Errorf("plain: Lang = %q, want empty", d.Lang)
	}
}

// TestNormalizeIndex 锁定入口文档识别: index / _index / README, 带语言后缀也识别.
func TestNormalizeIndex(t *testing.T) {
	res := normalizeFixture(t, map[string]string{
		"docs/index.md":        "# 首页\n",
		"docs/_index.md":       "# 下划线入口\n",
		"docs/guide/README.md": "# 目录说明\n",
		"docs/index.zh.md":     "# 中文首页\n",
		"docs/notindex.md":     "# 普通\n",
	})

	for _, rel := range []string{"docs/index.md", "docs/_index.md", "docs/guide/README.md", "docs/index.zh.md"} {
		if d, ok := testutil.Lookup(res.Docs, rel); ok && !d.IsIndex {
			t.Errorf("%s: IsIndex = false, want true", rel)
		}
	}
	// index.zh.md 的语言后缀照常剥离.
	if d, ok := testutil.Lookup(res.Docs, "docs/index.zh.md"); ok && (d.Lang != "zh" || d.Base != "index") {
		t.Errorf("index.zh: Lang=%q Base=%q, want zh/index", d.Lang, d.Base)
	}
	if d, ok := testutil.Lookup(res.Docs, "docs/notindex.md"); ok && d.IsIndex {
		t.Error("notindex.md: IsIndex = true, want false")
	}
}

// TestNormalizeTitle 锁定标题优先级: Front Matter title > 正文 H1 > 空.
func TestNormalizeTitle(t *testing.T) {
	res := normalizeFixture(t, map[string]string{
		"docs/with-fm.md":  "---\ntitle: 元数据标题\n---\n# H1 标题\n",
		"docs/with-h1.md":  "# 正文标题\n\n内容\n",
		"docs/no-title.md": "只是内容, 没有标题\n",
		"docs/h2-only.md":  "## 二级\n",
	})

	if d, ok := testutil.Lookup(res.Docs, "docs/with-fm.md"); ok && d.Title != "元数据标题" {
		t.Errorf("with-fm Title = %q, want 元数据标题 (FM 优先)", d.Title)
	}
	if d, ok := testutil.Lookup(res.Docs, "docs/with-h1.md"); ok && d.Title != "正文标题" {
		t.Errorf("with-h1 Title = %q, want 正文标题 (H1 兜底)", d.Title)
	}
	if d, ok := testutil.Lookup(res.Docs, "docs/no-title.md"); ok && d.Title != "" {
		t.Errorf("no-title Title = %q, want empty", d.Title)
	}
	// 只有 H2 时不能误当 H1 兜底.
	if d, ok := testutil.Lookup(res.Docs, "docs/h2-only.md"); ok && d.Title != "" {
		t.Errorf("h2-only Title = %q, want empty (H2 不作为标题兜底)", d.Title)
	}
}

// TestNormalizeSlug 锁定 slug 生成规则: 小写折叠/汉字保留/去首尾横线.
func TestNormalizeSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"intro", "intro"},
		{"My Post", "my-post"},
		{"Hello_World!", "hello-world"},
		{"我的文章", "我的文章"},
		{"---lead---", "lead"},
		{"v1.2", "v1-2"},
		{"", ""},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeSortsAndCarries 锁定排序与透传: 结果按 RelPath 排序, Space 透传.
func TestNormalizeSortsAndCarries(t *testing.T) {
	res := normalizeFixture(t, map[string]string{
		"docs/z.md": "# Z\n",
		"docs/a.md": "# A\n",
		"docs/m.md": "# M\n",
	})
	if res.Space == nil {
		t.Error("Space = nil, want 透传")
	}
	for i := 1; i < len(res.Docs); i++ {
		if res.Docs[i-1].RelPath > res.Docs[i].RelPath {
			t.Errorf("docs not sorted: %s > %s", res.Docs[i-1].RelPath, res.Docs[i].RelPath)
		}
	}
}

// TestNormalizeNilInput 锁定 nil 输入报错.
func TestNormalizeNilInput(t *testing.T) {
	if _, err := Normalize(context.Background(), nil); err == nil {
		t.Fatal("Normalize(context.Background(), nil): expected error, got nil")
	}
}
