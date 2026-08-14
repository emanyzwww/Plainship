// Package router 负责将源文件路径解析为 URL 路由与输出路径.
// 文件名, slug 与 URL 是三个独立概念, 必须解耦.
package router

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/model"
)

// Resolver 负责路由解析.
type Resolver struct {
	// base 是链接基础路径 (站点部署的子路径前缀), 例如 /repo 或空字符串.
	// 所有解析出的链接都以 base 开头, 保证任意页面深度下地址都指向正确的路由.
	base string
	// routes 是 sourceRel -> route 的映射, 例如 docs/foo.md -> foo/.
	// 由构建器在解析文档后填充, 供链接解析使用.
	routes map[string]string
	// assets 是 sourceRel -> 构建产物相对路径的映射,
	// 例如 docs/img/logo.png -> docs/img/logo.png.
	// 由构建器在扫描内容资源后填充, 供 Markdown 图片等非 md 链接解析使用.
	assets map[string]string
}

// New 创建路由解析器, 基础路径为空 (站点部署在域名根路径).
func New() *Resolver {
	return NewWithBase("")
}

// NewWithBase 创建路由解析器, base 是站点部署的子路径前缀.
// 例如部署在 https://example.com/repo 时 base 为 /repo, 域名根路径时为空字符串.
func NewWithBase(base string) *Resolver {
	return &Resolver{
		base:   strings.TrimRight(base, "/"),
		routes: map[string]string{},
		assets: map[string]string{},
	}
}

// Base 返回当前链接基础路径.
func (r *Resolver) Base() string {
	return r.base
}

// Register 登记一篇文档的路由.
// 在构建过程中逐步注册, 供后续链接解析使用.
func (r *Resolver) Register(sourceRel, route string) {
	r.routes[sourceRel] = route
}

// RegisterAsset 登记一个内容资源 (图片等非 md 文件).
// outputRel 是资源在构建产物中的相对路径.
func (r *Resolver) RegisterAsset(sourceRel, outputRel string) {
	r.assets[sourceRel] = outputRel
}

// Lookup 查询已登记的文档路由.
func (r *Resolver) Lookup(sourceRel string) (string, bool) {
	route, ok := r.routes[sourceRel]
	return route, ok
}

// RouteFor 计算一篇文档的路由与输出路径.
// slug 优先级: Front Matter slug > 文件基名(不含扩展名).
// 嵌套目录会保留在路由中.
// docs/<dir>/index.md 且未显式指定 slug 时作为该目录的索引页 (路由为 <dir>/).
// slug 必须是安全的相对路径段: 拒绝空段, "." , ".." 与反斜杠, 防止路径穿越.
// 返回 route 如 "hello-world/" 与 output 如 "hello-world/index.html".
func RouteFor(sourceRel string, meta model.Metadata) (route, output string, err error) {
	rel := strings.TrimPrefix(filepath.ToSlash(sourceRel), layout.DocsDir+"/")
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		dir = ""
	}
	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	slug := stem
	if meta != nil {
		if s := meta.GetString("slug"); s != "" {
			slug = s
		}
	}
	// 规范化 slug: 去除首尾斜杠与空白.
	rawSlug := slug
	slug = strings.Trim(slug, "/ ")
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = "index"
	}
	// 以 / 或  开头是绝对路径, 拒绝 (防止歧义与平台差异).
	if strings.HasPrefix(rawSlug, "/") || strings.HasPrefix(rawSlug, "\\") {
		return "", "", fmt.Errorf("invalid slug %q: must be a relative path without a leading slash", rawSlug)
	}
	if err := validateSlug(slug); err != nil {
		return "", "", err
	}
	// index.md 语义: 未显式指定 slug 的 docs/<dir>/index.md 作为目录索引页.
	if slug == stem && stem == "index" && dir != "" {
		slug = ""
	}
	parts := []string{}
	if dir != "" {
		parts = append(parts, dir)
	}
	if slug != "" {
		parts = append(parts, slug)
	}
	route = strings.Join(parts, "/") + "/"
	if route == "/" {
		// 理论不可达 (根目录 index.md 时 slug 回退为 "index"), 防御.
		route = "index/"
	}
	output = route + "index.html"
	return route, output, nil
}

// validateSlug 校验 slug 是安全的相对路径: 不允许空段, "." , ".." 或反斜杠.
func validateSlug(slug string) error {
	for _, part := range strings.Split(slug, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\\') {
			return fmt.Errorf("invalid slug %q: segments must not be empty, \".\", \"..\" or contain backslash", slug)
		}
	}
	return nil
}

// JoinURL 拼接基础路径与相对路径, 返回根相对 URL.
// 例如 JoinURL("/repo", "guide/foo/") -> "/repo/guide/foo/"; JoinURL("", "foo/") -> "/foo/".
// 模板的 url 函数与链接解析都使用它, 保证 dev 与正式环境下地址一致.
func JoinURL(base, p string) string {
	base = strings.TrimRight(base, "/")
	p = strings.TrimPrefix(p, "/")
	if base == "" {
		return "/" + p
	}
	return base + "/" + p
}

// ResolveLink 将 Markdown 链接目标解析为最终 URL.
// srcRel 是当前文档的源路径, dest 是链接中的原始目标.
// 规则:
//   - 以 .md 结尾的目标解析为对应文档的路由 (根相对 URL, 自动带基础路径前缀).
//   - 外部链接 (http/https///mailto:) 保持不变.
//   - 相对路径基于当前文档所在目录.
//   - 以 / 开头的路径基于 docs 根目录.
//   - 非 md 的相对路径: 若匹配已登记的内容资源则解析为资源 URL, 否则保持不变.
//   - 保留 fragment 与 query.
func (r *Resolver) ResolveLink(srcRel, dest string) string {
	if dest == "" {
		return dest
	}
	// 外部链接保持不变.
	lower := strings.ToLower(dest)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(dest, "//") || strings.HasPrefix(dest, "mailto:") {
		return dest
	}
	// 分离 fragment 与 query.
	pathPart := dest
	suffix := ""
	if idx := strings.IndexAny(pathPart, "#?"); idx >= 0 {
		pathPart = dest[:idx]
		suffix = dest[idx:]
	}
	if pathPart == "" {
		return dest
	}
	// 仅处理 Markdown 链接 (判断 pathPart 的后缀, 排除 #fragment 与 ?query).
	if strings.HasSuffix(strings.ToLower(pathPart), ".md") {
		// 计算目标源文件路径.
		var targetRel string
		if strings.HasPrefix(pathPart, "/") {
			targetRel = layout.DocsDir + "/" + strings.TrimPrefix(pathPart, "/")
		} else {
			// 相对路径基于当前文档目录.
			srcDir := filepath.ToSlash(filepath.Dir(srcRel))
			targetRel = filepath.ToSlash(filepath.Join(srcDir, pathPart))
		}
		// 统一为系统分隔符后再转正斜杠, 保证 key 一致.
		targetRel = filepath.ToSlash(filepath.Clean(targetRel))
		// 优先查找已登记路由 (含 docs/ 前缀形式, 兼容 ../ 越级后的路径).
		for _, cand := range []string{targetRel, docsPrefixed(targetRel)} {
			if route, ok := r.routes[cand]; ok {
				return JoinURL(r.base, route) + suffix
			}
			// 尝试 index 文档: guide/foo.md 找不到时尝试 guide/foo/index.md.
			if route, ok := r.routes[strings.TrimSuffix(cand, ".md")+"/index.md"]; ok {
				return JoinURL(r.base, route) + suffix
			}
		}
		// 兜底: 去掉 .md 扩展名并转成目录形式.
		fallback := strings.TrimSuffix(docsPrefixed(targetRel), ".md")
		fallback = strings.TrimPrefix(fallback, layout.DocsDir+"/")
		return JoinURL(r.base, fallback+"/") + suffix
	}
	// 以 / 开头的非 md 路径是作者写的根相对地址 (如 /assets/app.css), 保持不变.
	if strings.HasPrefix(pathPart, "/") {
		return dest
	}
	// 非 md 的相对路径: 尝试按源文件目录解析为内容资源 (图片等).
	srcDir := filepath.ToSlash(filepath.Dir(srcRel))
	targetRel := filepath.ToSlash(filepath.Clean(filepath.Join(srcDir, pathPart)))
	if out, ok := r.assets[targetRel]; ok {
		return JoinURL(r.base, out) + suffix
	}
	return dest
}

// docsPrefixed 返回带 docs/ 前缀的源路径 key.
// 相对链接经过 ../ 越级后可能丢失 docs/ 前缀, 补回以便命中登记路由.
func docsPrefixed(p string) string {
	if strings.HasPrefix(p, layout.DocsDir+"/") {
		return p
	}
	return layout.DocsDir + "/" + p
}

// EncodePath 对 URL 路径进行百分号编码, 用于 sitemap 等场景.
// 保留斜杠与合法的路径字符.
func EncodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
