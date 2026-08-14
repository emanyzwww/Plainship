// Package hash 提供内容哈希与构建输入哈希计算.
// 处于依赖最底层, 不依赖任何其他 internal 包.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
)

// File 计算单个文件的 SHA-256 哈希
// 返回十六进制字符串
func File(path string) (string, error) {
	// 打开文件
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// 创建 sha256 哈希对象
	h := sha256.New()

	// 写入哈希对象
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Bytes 计算字节内容的 SHA-256 哈希
// 返回十六进制字符串
func Bytes(b []byte) string {
	// 创建 sha256 哈希对象
	h := sha256.New()

	// 写入字节内容
	h.Write(b)

	return hex.EncodeToString(h.Sum(nil))
}

// String 计算字符串内容的 SHA-256 哈希
// 返回十六进制字符串
func String(s string) string {
	// 将字符串转换为字节内容
	return Bytes([]byte(s))
}

// BuildID 基于字符串内容生成构建 ID
// 返回哈希值的前 16 位
func BuildID(seed string) string {
	// 计算字符串哈希
	h := String(seed)

	return h[:16]
}

// Inputs 计算一组构建输入的联合哈希
// 返回十六进制字符串
func Inputs(inputs map[string]string) string {
	// 获取所有输入的 key
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}

	// 对 key 排序, 保证哈希结果稳定
	sort.Strings(keys)

	// 创建 sha256 哈希对象
	h := sha256.New()

	// 按排序后的 key 写入输入内容
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s\n", k, inputs[k])
	}

	return hex.EncodeToString(h.Sum(nil))
}
