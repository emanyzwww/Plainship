package pipeline

// Stage 描述管线中的一个阶段: 输入 In, 产出 Out.
//
// 各层 (scanner/parser/normalizer/assembly/...) 用 struct + Run 实现本接口后,
// 即可被编排层统一串联.
type Stage[In, Out any] interface {
	Run(in In) (Out, error)
}

// FuncStage 把普通函数适配为 Stage.
type FuncStage[In, Out any] func(In) (Out, error)

// Run 调用底层函数.
func (f FuncStage[In, Out]) Run(in In) (Out, error) { return f(in) }
