package pipeline

import "fmt"

// Problemf 构造一个 Problem, 消息按 fmt 格式.
func Problemf(sev Severity, stage, path, format string, args ...any) Problem {
	return Problem{
		Severity: sev,
		Stage:    stage,
		Path:     path,
		Message:  fmt.Sprintf(format, args...),
	}
}

// IsWarning 报告问题是否为 warning 级.
func (p Problem) IsWarning() bool { return p.Severity == SeverityWarning }

// IsError 报告问题是否为 error 级.
func (p Problem) IsError() bool { return p.Severity == SeverityError }

// Summary 是跨阶段问题的汇总统计.
type Summary struct {
	StageCount int // StageCount 参与统计的阶段数.
	Total      int // Total 问题总数.
	Warnings   int // Warnings warning 数.
	Errors     int // Errors error 数.
}

// Summarize 统计问题汇总.
func Summarize(problems []Problem) Summary {
	var s Summary
	s.Total = len(problems)
	for _, p := range problems {
		switch p.Severity {
		case SeverityWarning:
			s.Warnings++
		case SeverityError:
			s.Errors++
		}
	}
	return s
}

// GroupByStage 按问题来源层分组, 便于逐层展示.
func GroupByStage(problems []Problem) map[string][]Problem {
	m := make(map[string][]Problem)
	for _, p := range problems {
		m[p.Stage] = append(m[p.Stage], p)
	}
	return m
}

// MergeProblems 按顺序合并多个阶段的问题切片.
func MergeProblems(groups ...[]Problem) []Problem {
	var out []Problem
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
