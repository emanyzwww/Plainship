// Package fsutil 提供安全的文件系统操作工具.
// 重点处理路径规范化与遍历防护, 供构建与同步共用.
package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// SafeRelPath 校验并规范化相对路径.
// 拒绝绝对路径、空路径、包含 .. 或 drive 前缀的路径.
// 返回清理后的路径, 使用系统分隔符.
func SafeRelPath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", os.ErrInvalid
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if clean == "." {
		return "", os.ErrInvalid
	}
	if filepath.IsAbs(clean) {
		return "", os.ErrInvalid
	}
	vol := filepath.VolumeName(clean)
	if vol != "" {
		return "", os.ErrInvalid
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		strings.HasPrefix(clean, string(filepath.Separator)) {
		return "", os.ErrInvalid
	}
	return clean, nil
}

// SafeJoin 将相对路径安全地连接到 base 目录下.
// 如果 rel 包含越界路径, 返回错误.
func SafeJoin(base, rel string) (string, error) {
	clean, err := SafeRelPath(rel)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(base, clean)
	// 再次校验结果仍在 base 之下.
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(joinedAbs, baseAbs+string(filepath.Separator)) && joinedAbs != baseAbs {
		return "", os.ErrInvalid
	}
	return joined, nil
}

// EnsureDir 递归创建目录, 已存在则忽略.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// WriteFile 写入文件并确保父目录存在.
func WriteFile(path string, data []byte) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// RemoveAll 删除路径, 不存在时忽略错误.
func RemoveAll(path string) error {
	err := os.RemoveAll(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CopyFile 复制文件内容并保持权限.
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, data)
}

// CopyDir 递归复制目录内容到目标目录.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return EnsureDir(target)
		}
		return CopyFile(path, target)
	})
}

// ListFiles 递归列出目录下的所有文件, 返回相对路径列表.
func ListFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

// Exists 判断路径是否存在.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir 判断路径是否为目录.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
