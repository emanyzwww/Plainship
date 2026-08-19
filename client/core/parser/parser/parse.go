package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	"github.com/emanyzwww/papership-client/core/scanner/scanner"
)

// 可注入的测试替身.
var osReadFile = os.ReadFile

// bom 是 UTF-8 BOM, 解析前剥离, 但 Hash 仍基于原始内容计算.
var bom = []byte{0xEF, 0xBB, 0xBF}

// Parse 执行一次完整解析.
func Parse(scanned *scanner.Result) (*Result, error) {
	return ParseWithOptions(scanned, ParseOptions{})
}

// ParseWithOptions 与 Parse 相同, 支持自定义解析选项.
func ParseWithOptions(scanned *scanner.Result, _ ParseOptions) (*Result, error) {
	if scanned == nil {
		return nil, fmt.Errorf("parser: nil scan result")
	}

	res := &Result{Space: scanned.Space}
	for _, entry := range scanned.Docs {
		doc, problems, err := parseEntry(entry)
		if err != nil {
			res.Problems = append(res.Problems, scanner.Problem{
				Severity: scanner.SeverityError,
				Path:     entry.RelPath,
				Message:  "解析失败: " + err.Error(),
			})
			continue
		}
		res.Docs = append(res.Docs, doc)
		res.Problems = append(res.Problems, problems...)
	}

	sortDocs(res.Docs)
	return res, nil
}

// parseEntry 解析单个文档: 读文件, 切分 Front Matter, 解码元数据, 解析 AST.
//
// 返回的 problems 是该文档自身的非致命问题. 返回的 error 表示该文档无法生成.
func parseEntry(entry scanner.DocEntry) (Document, []scanner.Problem, error) {
	content, err := osReadFile(entry.AbsPath)
	if err != nil {
		return Document{}, nil, fmt.Errorf("读取文件失败: %w", err)
	}

	hash := hashContent(content)
	content = bytes.TrimPrefix(content, bom)

	metaRaw, body, has, closed := splitFrontMatter(content)
	var problems []scanner.Problem
	var meta map[string]any

	switch {
	case !has:
		meta = map[string]any{}
	case !closed:
		problems = append(problems, scanner.Problem{
			Severity: scanner.SeverityWarning,
			Path:     entry.RelPath,
			Message:  "Front Matter 缺少结尾分隔行 (---), 按无元数据处理",
		})
		meta = map[string]any{}
		body = content
	default:
		m, derr := decodeMeta(metaRaw)
		if derr != nil {
			problems = append(problems, scanner.Problem{
				Severity: scanner.SeverityError,
				Path:     entry.RelPath,
				Message:  "Front Matter 解析失败: " + derr.Error(),
			})
			meta = map[string]any{}
		} else {
			meta = m
		}
	}

	docAST := parseMarkdown(body)
	doc := Document{
		Entry: entry,
		Meta:  meta,
		AST:   docAST,
		Body:  body,
		Hash:  hash,
	}
	return doc, problems, nil
}

// hashContent 返回原始内容十六进制 SHA-256.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// sortDocs 按 RelPath 排序, 与 scanner 的排序约定一致.
func sortDocs(docs []Document) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].Entry.RelPath < docs[j].Entry.RelPath })
}
