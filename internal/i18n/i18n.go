// Package i18n 提供 Plainship 的中英双语支持 (英文优先, 中文可切换).
//
// 消息源: internal/i18n/locales/{en,zh}.yaml (每语言一个文件, 命名占位符 {{ .name }}).
// 消息键: 由 go generate 从 YAML 生成常量 (messages_gen.go), 拼写错误在编译期暴露.
// 使用:   i18n.T(i18n.CliNewOk, i18n.V{"detail": root}) 命名渲染;
//
//	i18n.T(i18n.CliNewOk, root)             位置渲染 (对应 {{ .arg0 }});
//	i18n.Errorf(i18n.CorePublishRejectDirty, i18n.V{"category": cat}) 本地化错误.
//
// 排版 (帮助文本的换行/对齐/缩进) 由 internal/format 负责, 消息文件保持纯文本.
//
// 两级语言:
//   - 工具语言 (CLI 输出与错误提示): 由 PLAINSHIP_LANG 环境变量或 --lang 参数决定.
//   - 站点语言 (生成网站的主题文案): 由 plainship.yaml 的 site.language 决定.
//
// 英文是默认工具语言 (英文优先), 中文可通过 PLAINSHIP_LANG / --lang 切换.
package i18n

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:generate go run ./gen

//go:embed locales/*.yaml
var localeFS embed.FS

// Lang 是工具语言.
type Lang string

const (
	// LangZH 简体中文.
	LangZH Lang = "zh"
	// LangEN 英文.
	LangEN Lang = "en"
)

// Parse 将语言字符串解析为 Lang.
// 支持各语言变体.
// 无法识别时回退到默认语言.
func Parse(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return DefaultLang()
	case "zh", "zh-cn", "zh_cn", "zh_hans", "zh-hans", "cn", "chinese", "中文":
		return LangZH
	case "en", "en-us", "en_gb", "english", "英文":
		return LangEN
	default:
		return DefaultLang()
	}
}

// DefaultLang 返回默认工具语言.
func DefaultLang() Lang {
	return LangEN
}

// Detect 从环境变量 PLAINSHIP_LANG 检测工具语言, 未设置时返回默认语言.
func Detect() Lang {
	return Parse(os.Getenv("PLAINSHIP_LANG"))
}

// String 返回语言的标准表示, 用于展示.
func (l Lang) String() string {
	switch l {
	case LangZH:
		return "中文"
	case LangEN:
		return "English"
	}
	return string(l)
}

// Code 返回语言的短代码, 例如 zh / en.
func (l Lang) Code() string {
	return string(l)
}

// IsEN 判断是否为英文.
func (l Lang) IsEN() bool {
	return l == LangEN
}

// V 是命名插值变量, 对应消息模板中的 {{ .name }}.
type V map[string]any

// L 是不可变的语言打印器: 一种语言一个实例, 模板解析一次后缓存, 并发安全.
type L struct {
	lang Lang
	tpls map[Key]*template.Template
}

// New 加载指定语言的打印器, 解析并缓存全部消息模板.
// 语言文件缺失或模板非法时返回错误.
func New(lang Lang) (*L, error) {
	lang = Parse(string(lang))
	data, err := localeFS.ReadFile("locales/" + lang.Code() + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("i18n: 加载语言 %s 失败: %w", lang, err)
	}
	raw := map[string]string{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("i18n: 解析语言 %s 失败: %w", lang, err)
	}
	l := &L{lang: lang, tpls: make(map[Key]*template.Template, len(raw))}
	for k, v := range raw {
		tpl, err := template.New(k).Parse(v)
		if err != nil {
			return nil, fmt.Errorf("i18n: 消息 %s 模板解析失败: %w", k, err)
		}
		l.tpls[Key(k)] = tpl
	}
	return l, nil
}

// MustNew 加载语言打印器, 失败时 panic.
func MustNew(lang Lang) *L {
	l, err := New(lang)
	if err != nil {
		panic(err)
	}
	return l
}

// Lang 返回打印器绑定的语言.
func (l *L) Lang() Lang {
	if l == nil {
		return DefaultLang()
	}
	return l.lang
}

// T 渲染消息.
// vars 支持三种形式:
//   - 无变量:     T(key)
//   - 命名变量:   T(key, V{"detail": x})
//   - 位置变量:   T(key, x, y)  对应模板中的 {{ .arg0 }} {{ .arg1 }}
//
// key 不存在或渲染失败时回退返回 key 本身.
func (l *L) T(key Key, vars ...any) string {
	if l == nil {
		return string(key)
	}
	tpl, ok := l.tpls[key]
	if !ok {
		return string(key)
	}
	var out strings.Builder
	if err := tpl.Execute(&out, normalizeVars(vars)); err != nil {
		return string(key)
	}
	return out.String()
}

// Has 判断消息键是否存在 (供主题加载期静态校验模板中的 t 引用).
func (l *L) Has(key Key) bool {
	if l == nil {
		return false
	}
	_, ok := l.tpls[key]
	return ok
}

// normalizeVars 将调用参数归一化为模板变量 map.
// 单参数且为 map 时视为命名变量: 支持 V (Go 侧) 与裸 map[string]any
// (主题模板通过 dict 函数构造); 其余情况按位置参数归一化为 arg0/arg1/....
func normalizeVars(vars []any) V {
	if len(vars) == 0 {
		return V{}
	}
	if len(vars) == 1 {
		switch v := vars[0].(type) {
		case V:
			return v
		case map[string]any:
			return V(v)
		}
	}
	m := make(V, len(vars))
	for i, v := range vars {
		m["arg"+fmt.Sprintf("%d", i)] = v
	}
	return m
}

// ---- 进程级默认打印器 ----
// 唯一的全局可变点, 由 CLI 在启动时设置一次 (原子替换, 并发安全).

var defaultL atomic.Pointer[L]

func init() {
	defaultL.Store(MustNew(DefaultLang()))
}

// SetDefault 设置全局默认打印器 (通常只在 CLI 启动时调用一次).
func SetDefault(l *L) {
	if l != nil {
		defaultL.Store(l)
	}
}

// SetLang 便捷方法: 加载指定语言并设为全局默认.
// 返回错误时全局默认保持不变.
func SetLang(lang Lang) error {
	l, err := New(lang)
	if err != nil {
		return err
	}
	SetDefault(l)
	return nil
}

// Default 返回全局默认打印器.
func Default() *L {
	return defaultL.Load()
}

// T 使用全局默认打印器渲染消息 (见 L.T).
func T(key Key, vars ...any) string {
	return Default().T(key, vars...)
}

// ---- 本地化错误 ----

// MsgError 是携带消息键与变量的本地化错误.
// 渲染发生在 Error() 调用时, 因此错误对象可以在任意语言下格式化.
type MsgError struct {
	Key   Key
	Vars  V
	Cause error
}

// Error 渲染错误消息 (使用全局默认语言).
func (e *MsgError) Error() string {
	if e == nil {
		return ""
	}
	msg := Default().T(e.Key, e.Vars)
	if e.Cause != nil {
		return msg + ": " + e.Cause.Error()
	}
	return msg
}

// Unwrap 返回底层错误 (支持 errors.Is / errors.As 链).
func (e *MsgError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Errorf 创建本地化错误.
// vars 与 L.T 相同: 支持命名 V 或位置参数.
func Errorf(key Key, vars ...any) error {
	return &MsgError{Key: key, Vars: normalizeVars(vars)}
}

// Wrapf 创建包装了底层错误的本地化错误.
func Wrapf(cause error, key Key, vars ...any) error {
	return &MsgError{Key: key, Vars: normalizeVars(vars), Cause: cause}
}

// RenderError 渲染错误消息 (nil 安全).
// MsgError.Error() 已递归包含 Cause 文本, 错误链无需在此展开;
// CLI 顶层统一用它把错误输出到 stderr.
func RenderError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
