package parser

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontMatterOpen 是 Front Matter 的开闭分隔行.
const frontMatterOpen = "---"

// splitFrontMatter 把原始内容切成 Front Matter 段与正文段.
//
// 规则:
//   - 仅当文件第一行是 --- 时视为存在 Front Matter, 随后找到第一个同样为 --- 的行作为闭合行.
//   - 正文中再出现的 --- 行不影响切分.
//
// 返回:
//   - meta: 闭合行之间的原始字节. 未闭合或无 Front Matter 时为 nil.
//   - body: 正文. 未闭合时整篇原文.
//   - has: 是否存在 Front Matter.
//   - closed: 是否找到闭合行. has 为 true 但 closed 为 false 表示未闭合.
func splitFrontMatter(content []byte) (meta, body []byte, has, closed bool) {
	n := len(content)

	// 无内容
	if n == 0 {
		return nil, content, false, false
	}

	// 第一行行尾, 无换行时即文件末尾.
	end, _ := lineEnd(content, 0)
	if strings.TrimSpace(string(content[:end])) != frontMatterOpen {
		return nil, content, false, false
	}

	// 从第二行起寻找闭合行.
	for i := end; i < n; {
		e, ok := lineEnd(content, i)
		if strings.TrimSpace(string(content[i:e])) == frontMatterOpen {
			return content[end:i], content[e:], true, true
		}
		if !ok {
			return nil, content, true, false
		}
		i = e
	}
	return nil, content, true, false
}

// lineEnd 返回从 start 开始当前行的行尾偏移.
//
// 返回的 ok 为 false 仅出现在 "start 已到文件末尾" 的情况. 文件最后一行若没有换行符,
// 行尾就是文件末尾, ok 仍为 true.
func lineEnd(content []byte, start int) (int, bool) {
	n := len(content)
	i := start
	if i >= n {
		return i, false
	}
	for i < n && content[i] != '\n' {
		i++
	}
	if i == n {
		return i, true // 最后一行无换行符.
	}
	return i + 1, true
}

// decodeMeta 把 Front Matter 字节解码为元数据 map.
//
// 空块得到空 map. 内容不是 YAML 映射时报错, 由调用方收集为 Problem 后按无元数据处理.
func decodeMeta(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}
