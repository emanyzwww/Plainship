package config

import (
	"os"
	"path/filepath"

	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
)

const FileName = layout.ConfigFile // FileName 是空间配置文件名.

// IsSpaceRoot 判断目录是否为 Plainship Space 根目录.
func IsSpaceRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, layout.ConfigFile))
	return err == nil && !info.IsDir()
}

// FindRoot 从 dir 向上逐级查找 Space 根目录.
//
// 找不到时返回错误.
func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	cur := abs
	for {
		if IsSpaceRoot(cur) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", i18n.Errorf(i18n.ConfigNotFound)
		}
		cur = parent
	}
}
