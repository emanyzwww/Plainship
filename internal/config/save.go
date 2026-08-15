package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// SaveTarget 是配置写入目标层.
type SaveTarget int

const (
	SaveGlobal  SaveTarget = 0 // SaveGlobal 写入全局配置 `~/.plainship/config.yaml`.
	SaveProject SaveTarget = 1 // SaveProject 写入空间级客户端配置 `<space>/.plainship/config.yaml`.
	SaveSpace   SaveTarget = 2 // SaveSpace 写入空间网站配置 `<space>/plainship.yaml`.
)

// Save 把 config 中指定目标域的配置项持久化到对应文件.
//
// 写入策略:
//   - 写出所有生效值, 文件始终是 config 的完整快照.
//   - 文件中的未知键随全量重写自然移除.
//   - SaveGlobal / SaveProject 只写显式设置过的项, SaveSpace 全量写出.
//
// 返回实际写入的文件路径.
func Save(config *Config, target SaveTarget) (string, error) {
	var path string
	var perm os.FileMode
	var domains []string
	switch target {
	case SaveGlobal:
		domains = []string{"GlobalClient", "GlobalServer"}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".plainship", "config.yaml")
		perm = 0o600
	case SaveProject:
		domains = []string{"SpaceClient"}
		if err := requireSpaceRoot(config); err != nil {
			return "", err
		}
		path = filepath.Join(spaceRootOf(config), ".plainship", "config.yaml")
		perm = 0o600
	case SaveSpace:
		domains = []string{"SpaceSite"}
		if err := requireSpaceRoot(config); err != nil {
			return "", err
		}
		path = filepath.Join(spaceRootOf(config), "plainship.yaml")
		perm = 0o644
	default:
		return "", os.ErrInvalid
	}

	nested := map[string]any{}
	rv := reflect.ValueOf(config).Elem()
	skipUnset := target != SaveSpace
	for _, name := range domains {
		collectNested(nested, rv.FieldByName(name), skipUnset)
	}
	data, err := yaml.Marshal(nested)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return "", err
	}
	return path, nil
}

// spaceRootOf 从 config 中取出空间根目录.
func spaceRootOf(config *Config) string {
	return config.spaceRoot
}

// requireSpaceRoot 校验空间根目录已设置.
func requireSpaceRoot(config *Config) error {
	if config.spaceRoot == "" {
		return fmt.Errorf("空间根目录未设置 (请先调用 SetSpaceRoot)")
	}
	return nil
}

// collectNested 把指定域的配置项收集进嵌套 map, skipUnset 为 true 时跳过未设置项.
func collectNested(nested map[string]any, domain reflect.Value, skipUnset bool) {
	walkItems(domain, func(it Item) {
		if skipUnset && !it.HasValue() {
			return
		}
		setNested(nested, it.Name(), it.Effective())
	})
}

// setNested 把扁平键写入嵌套 map.
func setNested(m map[string]any, key string, v any) {
	parts := strings.Split(key, ".")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = v
}
