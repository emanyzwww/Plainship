// Package output 是输出层入口: 把渲染结果写入输出目录, 生成完整静态站点与附加文件.
//
// 负责四类产出:
//   - 页面: 每页 HTML 按 clean URL 写盘,
//     如 guide/intro/ → <build>/guide/intro/index.html;
//   - 静态资源: docs 下的非文档文件 (去掉 docs 前缀) 与主题 static 目录原样拷贝;
//   - 附加文件: sitemap.xml / search-index.json / robots.txt;
//   - 逐文件失败收集为 Problem 并继续, 不中断整站出盘 (与全管线容错哲学一致).
package output

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/papership-client/core/derive"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/core/render"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/model/space"
)

// Input 是输出阶段的输入: 来源不同的各层产物在此汇合.
type Input struct {
	Space   *space.Space         // Space 本次构建的 Space.
	Theme   string               // Theme 实际使用的主题名 (来自 render).
	Pages   []render.Page        // Pages 渲染完成的页面.
	Assets  []scanner.AssetEntry // Assets 静态资源 (来自 scan).
	SiteMap []string             // SiteMap 全部页面 URL (来自 derive).
	Search  []derive.SearchEntry // Search 搜索索引 (来自 derive).
}

// Written 是写盘文件清单中的一条.
type Written struct {
	Path  string // Path 相对 BuildDir 的路径 (正斜杠).
	Bytes int64  // Bytes 写入字节数.
}

// Result 是一次 Write 的完整产物.
type Result struct {
	pipeline.Result[Written] // Docs 为写出的文件清单; Space 透传.
}

// Stage 是输出阶段: 实现 pipeline.Stage, 供编排层串联; 零值可用.
type Stage struct{}

// Run 执行一次带上下文的输出.
func (Stage) Run(ctx context.Context, in *Input) (*Result, error) {
	return Write(ctx, in)
}

// Write 把渲染结果与派生数据写入输出目录.
func Write(ctx context.Context, in *Input) (*Result, error) {
	if in == nil {
		return nil, fmt.Errorf("output: nil input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sp := in.Space
	if sp == nil {
		return nil, fmt.Errorf("output: nil space")
	}

	buildDir := sp.BuildDir()
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, fmt.Errorf("output: create build dir %q: %w", buildDir, err)
	}

	out := &Result{pipeline.Result[Written]{Space: sp}}
	var problems []pipeline.Problem

	// 1) 页面.
	for _, p := range in.Pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dst := filepath.Join(buildDir, filepath.FromSlash(p.OutPath))
		if err := writeFile(dst, p.HTML); err != nil {
			problems = append(problems, pipeline.Problemf(pipeline.SeverityError, "output", p.OutPath,
				"写入页面失败: %v", err))
			continue
		}
		out.Docs = append(out.Docs, Written{Path: p.OutPath, Bytes: int64(len(p.HTML))})
	}

	// 2) 静态资源: docs 下的非文档文件 (去掉 docs 前缀).
	docsRoot := sp.Layout.Docs
	if docsRoot == "" {
		docsRoot = "docs"
	}
	for _, a := range in.Assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rel := a.RelPath
		if strings.HasPrefix(rel, docsRoot+"/") {
			rel = strings.TrimPrefix(rel, docsRoot+"/")
		}
		data, err := os.ReadFile(a.AbsPath)
		if err != nil {
			problems = append(problems, pipeline.Problemf(pipeline.SeverityWarning, "output", a.RelPath,
				"读取静态资源失败: %v", err))
			continue
		}
		dst := filepath.Join(buildDir, filepath.FromSlash(rel))
		if err := writeFile(dst, data); err != nil {
			problems = append(problems, pipeline.Problemf(pipeline.SeverityWarning, "output", rel,
				"写入静态资源失败: %v", err))
			continue
		}
		out.Docs = append(out.Docs, Written{Path: rel, Bytes: int64(len(data))})
	}

	// 3) 主题 static 目录 (若有): themes/<theme>/static/* → <build>/*.
	if err := writeThemeStatic(ctx, sp, in.Theme, buildDir, &problems, out); err != nil {
		problems = append(problems, pipeline.Problemf(pipeline.SeverityError, "output", "themes",
			"拷贝主题静态资源失败: %v", err))
	}

	// 4) 附加文件.
	addExtras(ctx, sp, in, buildDir, &problems, out)

	out.Problems = problems
	return out, nil
}

// writeFile 写盘: 确保父目录存在后写入; 无副作用文件不打扰原目录.
func writeFile(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

// writeThemeStatic 拷贝主题 static 目录到 build 根, 保留目录结构.
func writeThemeStatic(ctx context.Context, sp *space.Space, theme, buildDir string, problems *[]pipeline.Problem, out *Result) error {
	themesDir := sp.Layout.Themes
	if themesDir == "" {
		themesDir = "themes"
	}
	if theme == "" {
		return nil
	}
	staticRoot := filepath.Join(sp.Root, themesDir, theme, "static")
	info, err := os.Stat(staticRoot)
	if err != nil || !info.IsDir() {
		return nil // 无 static 目录: 静默跳过.
	}
	return filepath.WalkDir(staticRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单个遍历失败不中断.
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(staticRoot, path)
		if rerr != nil {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			*problems = append(*problems, pipeline.Problemf(pipeline.SeverityWarning, "output", filepath.ToSlash(rel),
				"读取主题静态资源失败: %v", rerr))
			return nil
		}
		dst := filepath.Join(buildDir, rel)
		if werr := writeFile(dst, data); werr != nil {
			*problems = append(*problems, pipeline.Problemf(pipeline.SeverityWarning, "output", filepath.ToSlash(rel),
				"写入主题静态资源失败: %v", werr))
			return nil
		}
		out.Docs = append(out.Docs, Written{Path: filepath.ToSlash(rel), Bytes: int64(len(data))})
		return nil
	})
}

// addExtras 生成 sitemap.xml / search-index.json / robots.txt.
func addExtras(ctx context.Context, sp *space.Space, in *Input, buildDir string, problems *[]pipeline.Problem, out *Result) {
	if err := ctx.Err(); err != nil {
		return
	}
	siteURL := strings.TrimSuffix(sp.Config.SiteURL, "/")

	// sitemap.xml.
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, u := range in.SiteMap {
		loc := u
		if siteURL != "" {
			loc = siteURL + u
		}
		sb.WriteString(`<url><loc>`)
		sb.WriteString(html.EscapeString(loc))
		sb.WriteString(`</loc></url>`)
	}
	sb.WriteString(`</urlset>`)
	if err := writeFile(filepath.Join(buildDir, "sitemap.xml"), []byte(sb.String())); err != nil {
		*problems = append(*problems, pipeline.Problemf(pipeline.SeverityError, "output", "sitemap.xml",
			"写入 sitemap.xml 失败: %v", err))
	} else {
		out.Docs = append(out.Docs, Written{Path: "sitemap.xml", Bytes: int64(sb.Len())})
	}

	// search-index.json.
	data, err := json.MarshalIndent(in.Search, "", "  ")
	if err != nil {
		*problems = append(*problems, pipeline.Problemf(pipeline.SeverityError, "output", "search-index.json",
			"序列化搜索索引失败: %v", err))
	} else if err := writeFile(filepath.Join(buildDir, "search-index.json"), data); err != nil {
		*problems = append(*problems, pipeline.Problemf(pipeline.SeverityError, "output", "search-index.json",
			"写入 search-index.json 失败: %v", err))
	} else {
		out.Docs = append(out.Docs, Written{Path: "search-index.json", Bytes: int64(len(data))})
	}

	// robots.txt.
	var rb strings.Builder
	rb.WriteString("User-agent: *")
	rb.WriteString("Allow: /")
	if siteURL != "" {
		rb.WriteString(fmt.Sprintf("Sitemap: %s/sitemap.xml", siteURL))
	}
	if err := writeFile(filepath.Join(buildDir, "robots.txt"), []byte(rb.String())); err != nil {
		*problems = append(*problems, pipeline.Problemf(pipeline.SeverityError, "output", "robots.txt",
			"写入 robots.txt 失败: %v", err))
	} else {
		out.Docs = append(out.Docs, Written{Path: "robots.txt", Bytes: int64(rb.Len())})
	}
}
