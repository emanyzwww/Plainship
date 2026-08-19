package pipeline

import "github.com/emanyzwww/papership-client/model/space"

// NewResult 构造一个空的结果信封.
func NewResult[T any](s *space.Space) *Result[T] {
	return &Result[T]{Space: s}
}

// DocCount 返回本阶段文档数量.
func (r *Result[T]) DocCount() int { return len(r.Docs) }

// ProblemCount 返回本阶段收集的问题数量.
func (r *Result[T]) ProblemCount() int { return len(r.Problems) }

// AddProblems 追加一到多个问题.
func (r *Result[T]) AddProblems(problems ...Problem) {
	r.Problems = append(r.Problems, problems...)
}
