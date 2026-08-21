package pipeline

import "context"

// Stage 描述管线中的一个阶段: 输入 In, 产出 Out.
//
// 各层 (scanner/parser/normalizer/assembly/...) 用一个实现类型实现本接口,
// 编排层 (core/build) 依序串联并把各阶段问题合并汇总.
type Stage[In, Out any] interface {
	Run(ctx context.Context, in In) (Out, error)
}

// FuncStage 把普通函数适配为 Stage.
type FuncStage[In, Out any] func(ctx context.Context, in In) (Out, error)

// Run 调用底层函数.
func (f FuncStage[In, Out]) Run(ctx context.Context, in In) (Out, error) { return f(ctx, in) }
