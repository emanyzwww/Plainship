// Package theme 负责加载与渲染主题.
// 主题目录结构: theme.json, layouts/, assets/.
// 优先加载 Space 中的 themes/<name>, 缺失时回退到内嵌默认主题.
package theme

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template/parse"
	"time"

	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/router"
	"github.com/emanyzwww/plainship/internal/theme/embed"
	"github.com/emanyzwww/plainship/internal/version"
)

// Theme 表示一个已加载的主题.
type Theme struct {
	Name     string
	Version  string
	Layouts  map[string]*template.Template
	Assets   map[string][]byte // 相对路径 -> 内容
	HasOwnFS bool              // 是否来自 Space 目录
}

// metadata 是 theme.json 的结构.
type metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// funcs 是模板可用的自定义函数.
// printer 决定文案语言 (t), baseURL 决定链接基础路径 (url 函数).
func funcs(printer *i18n.L, baseURL string) template.FuncMap {
	return template.FuncMap{
		// t 按 key 取语言文案: {{t "theme.nav.home"}} 或 {{t "theme.article.author" .Page.Author}}.
		"t": func(key string, args ...any) string {
			return printer.T(i18n.Key(key), args...)
		},
		// dict 把键值对组装为 map, 供 t 传命名参数:
		// {{t "ThemeGreeting" (dict "name" .Page.Title "count" 3)}}.
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, i18n.Errorf(i18n.ThemeDictOdd, len(pairs))
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs); i += 2 {
				k, ok := pairs[i].(string)
				if !ok {
					return nil, i18n.Errorf(i18n.ThemeDictBadKey, pairs[i])
				}
				m[k] = pairs[i+1]
			}
			return m, nil
		},
		// formatDate 将日期格式化为站点语言: 中文 2026年8月13日, 英文 Aug 13, 2026.
		"formatDate": func(s string) string {
			return formatDate(printer.Lang(), s)
		},
		"join": strings.Join,
		// url 把路由/资源路径拼接为根相对 URL: {{url .Prev.Route}} -> /guide/foo/.
		// 站点部署在子路径 (site.url 含路径) 时自动带上基础路径前缀, dev 模式使用根路径.
		// 模板中的链接一律通过它生成, 保证任意页面深度与任意部署方式下地址正确.
		"url": func(p string) string {
			return router.JoinURL(baseURL, p)
		},
	}
}

// formatDate 按语言格式化日期, 支持 YYYY-MM-DD 与 RFC3339 等输入.
func formatDate(lang i18n.Lang, s string) string {
	if s == "" {
		return ""
	}
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05", "2006/01/02"}
	var t time.Time
	for _, f := range formats {
		if parsed, err := time.Parse(f, s); err == nil {
			t = parsed
			break
		}
	}
	if t.IsZero() {
		return s
	}
	if lang.IsEN() {
		return t.Format("Jan 2, 2006")
	}
	return fmt.Sprintf("%d年%d月%d日", t.Year(), t.Month(), t.Day())
}

// Load 从 Space 的 themes/<name> 目录加载主题.
// 如果目录不存在, 回退到内嵌默认主题.
// lang 决定模板函数 (t / formatDate) 的语言, baseURL 决定链接基础路径 (url 函数).
func Load(spaceRoot, name string, lang i18n.Lang, baseURL string) (*Theme, error) {
	if name == "" {
		name = "default"
	}
	dir := filepath.Join(spaceRoot, layout.ThemesDir, name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		t, err := loadFromFS(os.DirFS(dir), "", lang, baseURL)
		if err != nil {
			return nil, i18n.Errorf(i18n.ThemeLoadFail, name, err)
		}
		t.HasOwnFS = true
		if t.Name == "" {
			t.Name = name
		}
		return t, nil
	}
	// 回退内嵌主题.
	t, err := LoadEmbedded(lang, baseURL)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// LoadEmbedded 加载内嵌的默认主题.
// 内嵌主题版本跟随产品版本 (由 internal/version 提供), 用于构建指纹.
func LoadEmbedded(lang i18n.Lang, baseURL string) (*Theme, error) {
	t, err := loadFromFS(embed.FS, "default", lang, baseURL)
	if err != nil {
		return nil, err
	}
	t.Version = version.Version
	return t, nil
}

// joinPath 拼接 FS 内的路径, 兼容 root 为空的情况.
func joinPath(root, name string) string {
	if root == "" {
		return name
	}
	return root + "/" + name
}

// loadFromFS 从文件系统加载主题.
// root 为空表示文件系统根即主题根 (os.DirFS).
func loadFromFS(fsys fs.FS, root string, lang i18n.Lang, baseURL string) (*Theme, error) {
	t := &Theme{Layouts: map[string]*template.Template{}, Assets: map[string][]byte{}}
	// 读取 theme.json.
	metaBytes, err := fs.ReadFile(fsys, joinPath(root, "theme.json"))
	if err == nil {
		var meta metadata
		if err := json.Unmarshal(metaBytes, &meta); err == nil {
			t.Name = meta.Name
		}
	}
	// 解析 layouts.
	layoutDir := joinPath(root, "layouts")
	printer := i18n.MustNew(lang)
	tpl := template.New("").Funcs(funcs(printer, baseURL))
	entries, err := fs.ReadDir(fsys, layoutDir)
	if err != nil {
		return nil, i18n.Errorf(i18n.ThemeNoLayouts, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		data, err := fs.ReadFile(fsys, joinPath(layoutDir, name))
		if err != nil {
			return nil, err
		}
		// 使用完整文件名作为模板名, 避免与内容中的 {{define "..."}} 冲突.
		if _, err := tpl.New(name).Parse(string(data)); err != nil {
			return nil, i18n.Errorf(i18n.ThemeParseFail, name, err)
		}
	}
	if tpl.Lookup("home") == nil || tpl.Lookup("article") == nil {
		return nil, i18n.Errorf(i18n.ThemeNeedHomeArticle)
	}
	// 静态校验模板中 t 引用的消息键: 拼写错误在加载期暴露,
	// 而不是渲染期静默输出裸 key.
	if err := checkTemplateKeys(tpl, printer); err != nil {
		return nil, err
	}
	t.Layouts["*"] = tpl
	// 收集 assets.
	assetDir := joinPath(root, "assets")
	if entries, err := fs.ReadDir(fsys, assetDir); err == nil {
		_ = entries
		_ = fs.WalkDir(fsys, assetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel := path
			if root != "" {
				rel = strings.TrimPrefix(path, root+"/")
			}
			data, err := fs.ReadFile(fsys, path)
			if err != nil {
				return nil
			}
			t.Assets[rel] = data
			return nil
		})
	}
	return t, nil
}

// checkTemplateKeys 静态校验模板中 t 函数引用的消息键.
// 调用形式 {{t "key" ...}} 且第一个参数为字符串字面量时,
// 在主题加载期即校验键存在, 拼写错误不再于渲染期静默输出裸 key.
// 第一个参数为表达式 (变量) 时跳过, 由运行时回退语义兜底.
func checkTemplateKeys(tpl *template.Template, printer *i18n.L) error {
	for _, t := range tpl.Templates() {
		// 根模板 (template.New("")) 未 Parse, Tree 为空, 跳过.
		if t.Tree == nil {
			continue
		}
		if err := walkNodes(t.Tree.Root, t.Name(), printer); err != nil {
			return err
		}
	}
	return nil
}

// walkNodes 递归遍历模板 AST, 检查节点内的 t 调用.
func walkNodes(n parse.Node, tplName string, printer *i18n.L) error {
	if n == nil {
		return nil
	}
	switch node := n.(type) {
	case *parse.ActionNode:
		return checkPipeKeys(node.Pipe, tplName, printer)
	case *parse.IfNode:
		return checkBranchKeys(&node.BranchNode, tplName, printer)
	case *parse.RangeNode:
		return checkBranchKeys(&node.BranchNode, tplName, printer)
	case *parse.WithNode:
		return checkBranchKeys(&node.BranchNode, tplName, printer)
	case *parse.ListNode:
		if node == nil {
			return nil
		}
		for _, child := range node.Nodes {
			if err := walkNodes(child, tplName, printer); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkBranchKeys 检查 if/range/with 分支 (条件管道 + 两个分支列表).
func checkBranchKeys(b *parse.BranchNode, tplName string, printer *i18n.L) error {
	if b.Pipe != nil {
		if err := checkPipeKeys(b.Pipe, tplName, printer); err != nil {
			return err
		}
	}
	if err := walkNodes(b.List, tplName, printer); err != nil {
		return err
	}
	return walkNodes(b.ElseList, tplName, printer)
}

// checkPipeKeys 检查管道中的 t 调用: 第一个参数是函数标识 t,
// 第二个参数是字符串字面量时校验消息键存在.
func checkPipeKeys(p *parse.PipeNode, tplName string, printer *i18n.L) error {
	if p == nil {
		return nil
	}
	for _, c := range p.Cmds {
		if len(c.Args) < 2 {
			continue
		}
		id, ok := c.Args[0].(*parse.IdentifierNode)
		if !ok || id.Ident != "t" {
			continue
		}
		key, ok := c.Args[1].(*parse.StringNode)
		if !ok {
			continue
		}
		if !printer.Has(i18n.Key(key.Text)) {
			return i18n.Errorf(i18n.ThemeMissingKey, tplName, key.Text)
		}
	}
	return nil
}

// Render 使用指定布局渲染页面.
func (t *Theme) Render(layout string, data any) (string, error) {
	tpl, ok := t.Layouts["*"]
	if !ok {
		return "", i18n.Errorf(i18n.ThemeNotLoaded)
	}
	if tpl.Lookup(layout) == nil {
		layout = "article"
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, layout, data); err != nil {
		return "", i18n.Errorf(i18n.ThemeRenderFail, layout, err)
	}
	return buf.String(), nil
}

// HasLayout 判断主题是否提供指定布局.
func (t *Theme) HasLayout(layout string) bool {
	tpl, ok := t.Layouts["*"]
	if !ok {
		return false
	}
	return tpl.Lookup(layout) != nil
}

// WriteAssets 将主题资源写入目标目录.
func (t *Theme) WriteAssets(dst string) (int, error) {
	count := 0
	for rel, data := range t.Assets {
		path := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return count, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// CopyTo 将内嵌主题复制到目标目录.
// 用于 plainship new 生成 themes/default.
func CopyTo(dst string) error {
	return fs.WalkDir(embed.FS, "default", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "default")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(embed.FS, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
