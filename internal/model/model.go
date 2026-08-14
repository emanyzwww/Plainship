// Package model 定义 Plainship 的核心数据模型.
// 所有模块共享这些类型, 避免循环依赖.
package model

import (
	"html/template"
	"strconv"
	"time"
)

// Metadata 表示 Front Matter 解析结果.
// 使用 map 而不是固定 struct, 以便主题和插件未来可以读取自定义字段.
type Metadata map[string]any

// GetString 从 metadata 中读取字符串字段.
// 如果字段不存在或类型不符, 返回空字符串.
func (m Metadata) GetString(key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case int:
		return intToString(t)
	case float64:
		return floatToString(t)
	case time.Time:
		return t.Format("2006-01-02")
	default:
		return ""
	}
}

// GetBool 从 metadata 中读取布尔字段.
// 如果字段不存在, 返回 false.
func (m Metadata) GetBool(key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// Document 表示一篇解析完成的 Markdown 文档.
type Document struct {
	// SourcePath 是相对 Space 根目录的源文件路径, 例如 docs/测试文档.md.
	SourcePath string
	// FileName 是文件基名, 例如 测试文档.md.
	FileName string
	// Stem 是去除扩展名的文件基名, 例如 测试文档.
	Stem string
	// Dir 是源文件所在目录(相对 docs), 例如 guide 或 "".
	Dir string

	// 以下字段来自 Front Matter 或默认推导.
	Title   string
	Author  string
	Date    string
	Tag     string
	Slug    string
	Layout  string
	Draft   bool
	Meta    Metadata
	Summary string

	// Route 是解析后的 URL 路径, 例如 测试文档/ 或 hello-world/.
	Route string
	// OutputPath 是相对 dist 的输出路径, 例如 测试文档/index.html.
	OutputPath string

	// ContentHTML 是渲染后的 HTML 正文.
	ContentHTML template.HTML
	// RawContent 是去除 Front Matter 后的 Markdown 原文.
	RawContent string
	// Hash 是源文件内容哈希.
	Hash string
	// ModifiedTime 是文件修改时间.
	ModifiedTime string
}

// DocInfo 是文档列表页使用的轻量信息.
type DocInfo struct {
	Title   string
	Route   string
	Date    string
	Tag     string
	Source  string
	Stem    string
	Summary string
}

// Site 表示站点级配置信息, 提供给模板.
type Site struct {
	Title       string
	Description string
	URL         string
	Language    string
	// BaseURL 是站点部署的基础路径 (site.url 的路径部分), 例如 /repo 或空字符串.
	// 模板中的链接应通过 url 函数生成, 自动带上该前缀;
	// 站点部署在域名根路径时为空字符串.
	BaseURL string
}

// PageData 是模板渲染时使用的页面数据.
type PageData struct {
	Site  Site
	Page  *Document
	Docs  []DocInfo
	Dir   string
	Nav   []NavItem
	Prev  *DocInfo
	Next  *DocInfo
	Build string // 页面类型: home / article / list

	// T 是语言感知的消息函数, 主题模板通过 {{.T "key"}} 取文案.
	// 由 builder 注入, 绑定站点语言 (plainship.yaml site.language).
	T func(key string, args ...any) string
	// FormatDate 是语言感知的日期格式化函数, 绑定站点语言.
	FormatDate func(s string) string
}

// NavItem 是导航项.
type NavItem struct {
	Title string
	Route string
}

// BuildResult 是一次构建的结果摘要.
type BuildResult struct {
	BuildID string
	// ChangedPages 是重新渲染的页面数.
	ChangedPages int
	// CopiedPages 是复用缓存的页面数.
	CopiedPages int
	// DeletedPages 是检测到的删除页面数.
	DeletedPages int
	// AssetFiles 是构建产物中的静态文件数.
	AssetFiles int
	// BuildPath 是激活后的 dist 目录.
	BuildPath string
	// TotalFiles 是本次构建输出的文件总数.
	TotalFiles int
}

func intToString(v int) string {
	return strconv.Itoa(v)
}

func floatToString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
