// tokenfile.go 负责服务器访问令牌的生成与持久化.
// 令牌保存于数据目录下的 server.token 文件 (0600), 重启不改变.
package cli

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// tokenFileName 是数据目录中的令牌文件名.
const tokenFileName = "server.token"

// generateToken 生成新的访问令牌: ps_ 前缀 + 32 个十六进制字符 (128 bit 随机).
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ps_" + hex.EncodeToString(b), nil
}

// tokenFilePath 返回数据目录中的令牌文件路径.
func tokenFilePath(dataDir string) string {
	return filepath.Join(dataDir, tokenFileName)
}

// LoadToken 读取数据目录中的访问令牌.
// 文件不存在时返回 os.ErrNotExist.
func LoadToken(dataDir string) (string, error) {
	data, err := os.ReadFile(tokenFilePath(dataDir))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveToken 将访问令牌写入数据目录 (0600), 目录不存在时自动创建.
func SaveToken(dataDir, token string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(tokenFilePath(dataDir), []byte(token+"\n"), 0o600)
}

// LoadOrCreateToken 读取令牌; 文件不存在时生成新令牌并持久化.
// 返回令牌与是否为新生成.
func LoadOrCreateToken(dataDir string) (string, bool, error) {
	t, err := LoadToken(dataDir)
	if err == nil {
		return t, false, nil
	}
	if !os.IsNotExist(err) {
		return "", false, err
	}
	t, err = generateToken()
	if err != nil {
		return "", false, err
	}
	if err := SaveToken(dataDir, t); err != nil {
		return "", false, err
	}
	return t, true, nil
}
