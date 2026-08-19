// Package pipeline 提供全管线共享的契约与基础设施:
//
//   - Problem / Severity: 从扫描到渲染各层统一的问题形态, 供逐层汇总与展示;
//   - Doc: 全管线共享的文档脊柱 (身份 + 语义字段, 各层按生命周期填充);
//   - Result[T]: 通用结果信封 (Space 透传 + Docs + Problems);
//   - 排序 / 问题统计 / Stage 契约: 让各层不再重复实现。
//
// 设计原则: 基础设施与稳定脊柱放在这里一次定义, 各层只挂自己的载荷, 不重复声明.
package pipeline

import "github.com/emanyzwww/papership-client/model/space"

// Severity 是问题严重级别.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Problem 记录管线中某个阶段产生的非致命问题.
type Problem struct {
	Severity Severity // Severity 严重级别.
	Stage    string   // Stage 问题来源层.
	Path     string   // Path 关联的路径.
	Message  string   // Message 人类可读的描述.
}

// Doc 是全管线共享的文档脊柱.
//
// 下游各层通过内嵌本类型获得统一字段, 不再各自重新声明.
type Doc struct {
	RelPath string // RelPath 相对 Space 根目录的路径.
	Dir     string // Dir 相对 docs 根目录的目录部分; 顶层文档为空.
	Stem    string // Stem 文件名 (不含扩展名), 如 "intro.zh".
	Base    string // Base 剥离语言后缀后的基名, 如 "intro"; 由 normalizer 推导.
	Lang    string // Lang 语言码, 如 "zh"; 无后缀为空; 由 normalizer 推导.
	Ext     string // Ext 扩展名, 小写, 含点, 如 ".md".
	Title   string // Title 标准化标题 (FM title 优先, H1 兜底); 由 normalizer 填写.
	Slug    string // Slug 用于 URL 的稳定标识; 由 normalizer 生成.
	Hash    string // Hash 原始文件内容 SHA-256, 供增量/缓存比对.
	IsIndex bool   // IsIndex 是否为入口文档 (index/_index/README); 由 normalizer 推导.
	Size    int64  // Size 文件字节数.
	ModTime int64  // ModTime 修改时间, 供增量扫描比对.
}

// Result 是一次管线阶段的通用结果信封: Space 透传 + 产物 Docs + 收集的问题.
type Result[T any] struct {
	Space    *space.Space // Space 本次处理的 Space, 逐层透传.
	Docs     []T          // Docs 本阶段产物.
	Problems []Problem    // Problems 本阶段收集的问题.
}
