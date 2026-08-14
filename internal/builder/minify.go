// minify.go 生产构建产物的压缩 (HTML/CSS/JS).
// 仅 plainship build 调用; dev 构建保持可读输出, 便于调试与热更新.
// minify 保留 <pre>/<textarea>/<script>/<style> 内部内容, 代码高亮块不受影响.
package builder

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/hash"
	"github.com/emanyzwww/plainship/internal/manifest"
)

// minifyDir 压缩目录下全部 HTML/CSS/JS 文件, 并刷新清单哈希.
// 其余文件 (robots.txt / sitemap.xml / 图片等) 原样保留.
// 单个文件压缩失败时跳过 (防御: 不因压缩问题阻塞构建).
func minifyDir(dir string, m *manifest.Manifest) error {
	files, err := fsutil.ListFiles(dir)
	if err != nil {
		return err
	}
	mn := minify.New()
	// KeepQuotes: 保留属性引号; KeepEndTags: 保留可省闭合标签 (如 </p>),
	// 避免依赖 HTML5 隐式闭合, 输出更接近常规 HTML.
	mn.AddFunc("text/html", (&html.Minifier{KeepQuotes: true, KeepEndTags: true}).Minify)
	mn.AddFunc("text/css", css.Minify)
	mn.AddFunc("application/javascript", js.Minify)
	for _, rel := range files {
		if !isMinifiable(rel) {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := minifyFile(mn, path, rel); err != nil {
			continue
		}
	}
	// 产物内容已变, 刷新清单哈希, 保证 publish/sync 的指纹一致.
	return refreshHashes(dir, m)
}

// isMinifiable 判断文件扩展名是否可压缩.
func isMinifiable(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".html", ".css", ".js":
		return true
	}
	return false
}

// minifyFile 压缩单个文件 (按扩展名选择 minifier).
func minifyFile(mn *minify.M, path, rel string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".html":
		err = mn.Minify("text/html", &buf, bytes.NewReader(data))
		if err == nil {
			// minify 解码属性值实体但只转义引号, 会把 &lt; 还原成裸 <.
			// 虽然引号属性值内的 < 不构成标签, 但为保证 unsafe=false 的
			// 语义 (输出不含可执行标签), 将其重新转义.
			raw := buf.Bytes()
			buf.Reset()
			buf.Write(protectAttrLt(raw))
		}
	case ".css":
		err = mn.Minify("text/css", &buf, bytes.NewReader(data))
	case ".js":
		err = mn.Minify("application/javascript", &buf, bytes.NewReader(data))
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// attrValuePattern 匹配带引号的属性值.
var attrValuePattern = regexp.MustCompile("\"[^\"]*\"|'[^']*'")

// protectAttrLt 把引号属性值内的裸 < 重新转义为 &lt;.
// minify 会解码属性值实体 (如 &lt; -> <) 且只转义引号;
// 后处理把 < 转回, 恢复"转义内容不直通"的语义保证.
func protectAttrLt(b []byte) []byte {
	return attrValuePattern.ReplaceAllFunc(b, func(m []byte) []byte {
		val := m[1 : len(m)-1]
		if !bytes.Contains(val, []byte("<")) {
			return m
		}
		out := make([]byte, 0, len(m)+4)
		out = append(out, m[0])
		out = append(out, bytes.ReplaceAll(val, []byte("<"), []byte("&lt;"))...)
		out = append(out, m[len(m)-1])
		return out
	})
}

// refreshHashes 按磁盘上的实际内容刷新清单中所有条目的哈希.
// 在 minify 之后调用, 保证清单与产物一致.
func refreshHashes(dir string, m *manifest.Manifest) error {
	for i := range m.Files {
		path := filepath.Join(dir, filepath.FromSlash(m.Files[i].Output))
		h, err := hash.File(path)
		if err != nil {
			return err
		}
		m.Files[i].Hash = h
	}
	return nil
}
