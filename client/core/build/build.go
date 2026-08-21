// Package build 串联执行解析管线各阶段, 汇总跨阶段问题, 产出最终构建结果.
//
// 这是管线编排的入口: scan → parse → normalize → assemble.
// 后续 derive/render/output 按相同方式接入 (每层产出并入 Problems 汇总).
package build

import (
	"context"

	"github.com/emanyzwww/papership-client/core/assembly"
	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/derive"
	"github.com/emanyzwww/papership-client/core/output"
	"github.com/emanyzwww/papership-client/core/parser/normalizer"
	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/core/render"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/model/space"
)

// 各阶段以 pipeline.Stage 接口声明; 编译期校验实现, Run 依序串联.
// 新阶段 (derive/render/output) 落地后按同样方式接入.
var (
	_ pipeline.Stage[*space.Space, *scanner.Result]                           = scanner.Stage{}
	_ pipeline.Stage[*scanner.Result, *parser.Result]                         = parser.Stage{}
	_ pipeline.Stage[*parser.Result, *normalizer.Result]                      = normalizer.Stage{}
	_ pipeline.Stage[*normalizer.Result, *pipeline.Result[document.Document]] = assembly.Stage{}
	_ pipeline.Stage[*pipeline.Result[document.Document], *derive.Result]     = derive.Stage{}
)

// 各阶段标记, 与 pipeline.Problem.Stage 对应, 用于逐层汇总展示.
const (
	StageScanner    = "scanner"
	StageParser     = "parser"
	StageNormalizer = "normalizer"
	StageAssembly   = "assembly"
)

// Result 是一次完整构建的产物: 当前终端产物 (写盘清单) + 跨阶段问题汇总.
type Result struct {
	Space    *space.Space
	Derived  *derive.Result     // Derived 派生结果 (Pages/Nav/SiteMap/SearchIndex).
	Rendered *render.Result     // Rendered 渲染结果 (每页 HTML + OutPath).
	Written  *output.Result     // Written 输出结果 (写盘清单 + 附加文件).
	Summary  pipeline.Summary   // Summary 跨阶段问题汇总.
	Problems []pipeline.Problem // Problems 按阶段顺序合并的全部问题.
}

// DocCount 返回写盘文件数量.
func (r *Result) DocCount() int {
	if r.Written == nil {
		return 0
	}
	return r.Written.DocCount()
}

// ProblemsByStage 按阶段分组的问题, 便于界面逐层展示.
func (r *Result) ProblemsByStage() map[string][]pipeline.Problem {
	return pipeline.GroupByStage(r.Problems)
}

// Run 执行 scan → parse → normalize → assemble → derive → render,
// 并把各阶段问题合并为 summary.
//
// 各阶段经 pipeline.Stage 接口串联; 返回的 error 仅代表整个管线无法继续
// (如 nil 输入 / 根级扫描错误 / 上下文取消); 单文件级问题一律进入 Result.Problems,
// 不中断构建.
func Run(ctx context.Context, s *space.Space) (*Result, error) {
	scanned, err := scanner.Stage{}.Run(ctx, s)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	parsed, err := parser.Stage{}.Run(ctx, scanned)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalized, err := normalizer.Stage{}.Run(ctx, parsed)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	assembled, err := assembly.Stage{}.Run(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	derived, err := derive.Stage{}.Run(ctx, assembled)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rendered, err := render.Stage{}.Run(ctx, derived)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	written, err := output.Stage{}.Run(ctx, &output.Input{
		Space:   s,
		Theme:   rendered.Theme,
		Pages:   rendered.Docs,
		Assets:  scanned.Assets,
		SiteMap: derived.SiteMap,
		Search:  derived.SearchIndex,
	})
	if err != nil {
		return nil, err
	}

	problems := pipeline.MergeProblems(
		scanned.Problems,
		parsed.Problems,
		normalized.Problems,
		assembled.Problems,
		derived.Problems,
		rendered.Problems,
		written.Problems,
	)
	summary := pipeline.Summarize(problems)
	summary.StageCount = 7

	return &Result{
		Space:    s,
		Derived:  derived,
		Rendered: rendered,
		Written:  written,
		Summary:  summary,
		Problems: problems,
	}, nil
}
