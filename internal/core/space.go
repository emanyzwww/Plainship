package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/ui"
)

// CreateSpace 在指定目录创建新的 Space, 默认初始化 Git.
//
// u 是输出入口, 接收 Git 缺失等警告; nil 表示静默.
func CreateSpace(root string, u ui.UI) (*space.Space, error) {
	return space.Create(root, u)
}

// CreateDocument 在 Space 中创建一篇 Markdown 文档.
// name 是文档名称, 支持嵌套目录, 例如 guide/快速开始.
func CreateDocument(spaceRoot, name string) (string, error) {
	s, err := space.Load(spaceRoot)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", i18n.Errorf(i18n.CoreSpaceNameEmpty)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	rel, err := fsutil.SafeRelPath(name)
	if err != nil {
		return "", i18n.Errorf(i18n.CoreSpaceNameInvalid, name)
	}
	target := filepath.Join(s.DocsDir(), filepath.FromSlash(rel))
	if fsutil.Exists(target) {
		return "", i18n.Errorf(i18n.CoreSpaceExists, filepath.ToSlash(rel))
	}
	title := filepath.Base(strings.TrimSuffix(rel, ".md"))
	content := fmt.Sprintf("---\ntitle: %s\nauthor:\ndate:\ntag:\n---\n", title)
	if err := fsutil.WriteFile(target, []byte(content)); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
