package pipeline

import (
	"strings"
	"testing"

	"github.com/emanyzwww/papership-client/model/space"
)

// TestResultEnvelope 锁定 Result[T] 信封: 计数方法 / 问题追加 / Space 透传.
func TestResultEnvelope(t *testing.T) {
	sp := &space.Space{Root: "/tmp/site"}
	res := NewResult[Doc](sp)
	if res.DocCount() != 0 || res.ProblemCount() != 0 {
		t.Fatalf("fresh result: DocCount=%d ProblemCount=%d, want 0/0", res.DocCount(), res.ProblemCount())
	}
	res.Docs = append(res.Docs, Doc{RelPath: "a.md"}, Doc{RelPath: "b.md"})
	res.AddProblems(
		Problem{Severity: SeverityWarning, Stage: "test", Path: "a.md", Message: "w"},
		Problem{Severity: SeverityError, Stage: "test", Path: "b.md", Message: "e"},
	)
	if res.DocCount() != 2 || res.ProblemCount() != 2 {
		t.Errorf("after add: DocCount=%d ProblemCount=%d, want 2/2", res.DocCount(), res.ProblemCount())
	}
	if res.Space != sp {
		t.Error("Space 未透传")
	}
}

// sortCase 内嵌脊柱的包装类型, 验证下游类型天然满足 Keyed.
type sortCase struct {
	Doc
	Extra string
}

// TestSortByKey 锁定通用排序: 直接使用 Doc 与内嵌 Doc 的类型都能排序.
func TestSortByKey(t *testing.T) {
	docs := []Doc{
		{RelPath: "docs/z.md"},
		{RelPath: "docs/a.md"},
		{RelPath: "docs/m.md"},
	}
	SortByKey(docs)
	want := []string{"docs/a.md", "docs/m.md", "docs/z.md"}
	for i, d := range docs {
		if d.RelPath != want[i] {
			t.Errorf("docs[%d].RelPath = %q, want %q", i, d.RelPath, want[i])
		}
	}

	cases := []sortCase{
		{Doc: Doc{RelPath: "b"}, Extra: "2"},
		{Doc: Doc{RelPath: "a"}, Extra: "1"},
	}
	SortByKey(cases)
	if cases[0].RelPath != "a" || cases[1].RelPath != "b" {
		t.Errorf("embedded sort broken: %+v", cases)
	}
	if cases[0].Key() != "a" {
		t.Errorf("Key() promotion broken: %q", cases[0].Key())
	}
}

// TestProblemHelpers 锁定 Problem 构造与判定.
func TestProblemHelpers(t *testing.T) {
	p := Problemf(SeverityWarning, "scanner", "docs/index.md", "missing dir %s", "themes")
	if !p.IsWarning() || p.IsError() {
		t.Errorf("severity flags wrong: %+v", p)
	}
	if p.Message != "missing dir themes" {
		t.Errorf("Message = %q, want format applied", p.Message)
	}
	if p.Stage != "scanner" || p.Path != "docs/index.md" {
		t.Errorf("fields wrong: %+v", p)
	}
}

// TestSummaryAndGrouping 锁定汇总统计与按阶段分组.
func TestSummaryAndGrouping(t *testing.T) {
	ps := []Problem{
		{Severity: SeverityWarning, Stage: "scanner", Path: "p1", Message: "w1"},
		{Severity: SeverityError, Stage: "scanner", Path: "p2", Message: "e1"},
		{Severity: SeverityWarning, Stage: "parser", Path: "p3", Message: "w2"},
		{Severity: SeverityError, Stage: "parser", Path: "p4", Message: "e2"},
	}
	s := Summarize(ps)
	if s.Total != 4 || s.Warnings != 2 || s.Errors != 2 {
		t.Errorf("Summarize = %+v, want Total=4 Warnings=2 Errors=2", s)
	}
	by := GroupByStage(ps)
	if len(by["scanner"]) != 2 || len(by["parser"]) != 2 {
		t.Errorf("GroupByStage wrong: %+v", by)
	}
	merged := MergeProblems(ps[:2], ps[2:])
	if len(merged) != 4 || merged[0].Path != "p1" || merged[3].Path != "p4" {
		t.Errorf("MergeProblems wrong: %+v", merged)
	}
}

// addOne 是适配成 Stage 的普通函数.
func addOne(n int) (int, error) { return n + 1, nil }

// TestFuncStage 锁定函数式 Stage 适配.
func TestFuncStage(t *testing.T) {
	st := FuncStage[int, int](addOne)
	out, err := st.Run(41)
	if err != nil || out != 42 {
		t.Errorf("FuncStage.Run = %d/%v, want 42/nil", out, err)
	}
}

// TestDocKey 锁定脊柱排序键.
func TestDocKey(t *testing.T) {
	d := Doc{RelPath: "docs/a/b.md"}
	if d.Key() != "docs/a/b.md" {
		t.Errorf("Key() = %q", d.Key())
	}
	if !strings.HasPrefix(d.Key(), "docs/") {
		t.Error("Key() should be space-relative path")
	}
}
