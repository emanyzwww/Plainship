package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
)

// 可注入的测试替身.
var osReadFile = os.ReadFile

// bom 是 UTF-8 BOM, 解析前剥离, 但 Hash 仍基于原始内容计算.
var bom = []byte{0xEF, 0xBB, 0xBF}

// problem 构造带解析层标记的共享问题.
func problem(sev pipeline.Severity, path, msg string) pipeline.Problem {
	return pipeline.Problem{Severity: sev, Stage: "parser", Path: path, Message: msg}
}

// Parse 执行一次完整解析.
func Parse(scanned *scanner.Result) (*Result, error) {
	return ParseWithOptions(scanned, ParseOptions{})
}

// ParseWithOptions 与 Parse 相同, 支持自定义解析选项.
func ParseWithOptions(scanned *scanner.Result, _ ParseOptions) (*Result, error) {
	if scanned == nil {
		return nil, fmt.Errorf("parser: nil scan result")
	}

	res := pipeline.NewResult[Document](scanned.Space)
	for _, entry := range scanned.Docs {
		doc, problems, err := parseEntry(entry)
		if err != nil {
			res.Problems = append(res.Problems, problem(pipeline.SeverityError, entry.RelPath, "解析失败: "+err.Error()))
			continue
		}
		res.Docs = append(res.Docs, doc)
		res.Problems = append(res.Problems, problems...)
	}

	pipeline.SortByKey(res.Docs)
	return res, nil
}

// parseEntry 解析单个文档: 读文件, 切分 Front Matter, 解码元数据, 解析 AST.
//
// 返回的 problems 是该文档自身的非致命问题. 返回的 error 表示该文档无法生成.
func parseEntry(entry scanner.DocEntry) (Document, []pipeline.Problem, error) {
	content, err := osReadFile(entry.AbsPath)
	if err != nil {
		return Document{}, nil, fmt.Errorf("读取文件失败: %w", err)
	}

	hash := hashContent(content)
	content = bytes.TrimPrefix(content, bom)

	metaRaw, body, has, closed := splitFrontMatter(content)
	var problems []pipeline.Problem
	var meta map[string]any

	switch {
	case !has:
		meta = map[string]any{}
	case !closed:
		problems = append(problems, problem(pipeline.SeverityWarning, entry.RelPath, "Front Matter 缺少结尾分隔行 (---), 按无元数据处理"))
		meta = map[string]any{}
		body = content
	default:
		m, derr := decodeMeta(metaRaw)
		if derr != nil {
			problems = append(problems, problem(pipeline.SeverityError, entry.RelPath, "Front Matter 解析失败: "+derr.Error()))
			meta = map[string]any{}
		} else {
			meta = m
		}
	}

	docAST := parseMarkdown(body)
	doc := Document{
		Doc: pipeline.Doc{
			RelPath: entry.RelPath,
			Dir:     entry.Dir,
			Stem:    entry.Stem,
			Ext:     entry.Ext,
			Size:    entry.Size,
			ModTime: entry.ModTime,
			Hash:    hash,
		},
		Meta: meta,
		AST:  docAST,
		Body: body,
	}
	return doc, problems, nil
}

// hashContent 返回原始内容十六进制 SHA-256.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
