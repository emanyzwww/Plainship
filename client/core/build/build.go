// Package build 串联执行解析管线各阶段, 汇总跨阶段问题, 产出最终构建结果.
//
// 这是管线编排的入口: scan → parse → normalize. 后续 assembly/derive/render
// 只需按 pipeline.Stage 接口接入, 由这里统一串联.
package build

import (
	"github.com/emanyzwww/papership-client/core/parser/normalizer"
	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/model/space"
)

// 各阶段标记, 与 pipeline.Problem.Stage 对应, 用于逐层汇总展示.
const (
	StageScanner    = "scanner"
	StageParser     = "parser"
	StageNormalizer = "normalizer"
)

// Result 是一次完整构建的产物: 最终文档 + 跨阶段问题汇总.
type Result struct {
	Space    *space.Space
	Docs     []parser.Document
	Summary  pipeline.Summary   // Summary 跨阶段问题汇总.
	Problems []pipeline.Problem // Problems 按阶段顺序合并的全部问题.
}

// DocCount 返回标准化后的文档数量.
func (r *Result) DocCount() int { return len(r.Docs) }

// ProblemsByStage 按阶段分组的问题, 便于界面逐层展示.
func (r *Result) ProblemsByStage() map[string][]pipeline.Problem {
	return pipeline.GroupByStage(r.Problems)
}

// Run 执行 scan → parse → normalize 并把各阶段问题合并为 summary.
//
// 返回的 error 仅代表整个管线无法继续 (如 nil 输入 / 根级扫描错误);
// 单文件级问题一律进入 Result.Problems, 不中断构建.
func Run(s *space.Space) (*Result, error) {
	scanned, err := scanner.Scan(s)
	if err != nil {
		return nil, err
	}

	parsed, err := parser.Parse(scanned)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizer.Normalize(parsed)
	if err != nil {
		return nil, err
	}

	problems := pipeline.MergeProblems(scanned.Problems, parsed.Problems, normalized.Problems)
	summary := pipeline.Summarize(problems)
	summary.StageCount = 3

	return &Result{
		Space:    s,
		Docs:     normalized.Docs,
		Summary:  summary,
		Problems: problems,
	}, nil
}
