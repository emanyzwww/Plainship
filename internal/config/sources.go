package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// envValues 读取环境变量层.
//
// 规则: 键名转大写并替换 `.` 为 `_`, 加 `PLAINSHIP_` 前缀.
//
// 例如: `lang` -> `PLAINSHIP_LANG`; `color` -> `PLAINSHIP_COLOR`.
//
// 环境变量只作用于客户端工具配置, 即 GlobalClient / SpaceClient 域;
// 空间网站配置 SpaceSite 域不支持环境变量;
// 空间敏感配置 `server.token` 不提供环境变量来源.
func envValues() map[string]string {
	out := map[string]string{}
	for _, key := range clientKeys() {
		if key == "server.token" {
			continue
		}
		env := "PLAINSHIP_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if v, ok := os.LookupEnv(env); ok {
			out[key] = v
		}
	}
	return out
}

// clientKeys 返回客户端工具配置的去重键名, 来自 GlobalClient + SpaceClient 域.
func clientKeys() []string {
	seen := map[string]bool{}
	var keys []string
	for _, name := range []string{"GlobalClient", "SpaceClient"} {
		walkItems(reflect.ValueOf(Default()).Elem().FieldByName(name), func(it Item) {
			if !seen[it.Name()] {
				seen[it.Name()] = true
				keys = append(keys, it.Name())
			}
		})
	}
	return keys
}

// globalValues 读取全局配置 `~/.plainship/config.yaml`.
//
// 文件不存在时返回空 map.
func globalValues() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return readYAMLFlat(filepath.Join(home, ".plainship", "config.yaml"))
}

// projectValues 读取空间级客户端配置 `<root>/.plainship/config.yaml`.
func projectValues(root string) (map[string]string, error) {
	return readYAMLFlat(filepath.Join(root, ".plainship", "config.yaml"))
}

// spaceValues 读取空间网站配置 `<root>/plainship.yaml`.
func spaceValues(root string) (map[string]string, error) {
	return readYAMLFlat(filepath.Join(root, "plainship.yaml"))
}

// readYAMLFlat 读取 YAML 文件并扁平化为 key -> 字符串 map.
//
// 嵌套 map 用 `.` 连接, 如 `site.title`.
//
// 文件不存在时返回空 map, 文件损坏时返回错误.
func readYAMLFlat(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("配置文件损坏 %s: %w", path, err)
	}
	flat := map[string]string{}
	flattenYAML("", m, flat)
	return flat, nil
}

// flattenYAML 递归扁平化 YAML map.
func flattenYAML(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenYAML(key, val, out)
		case string, bool, int, int64, float64:
			out[key] = fmt.Sprint(val)
		}
	}
}

// SourceMap 返回指定层的原始值 map, 供精确查看某一层使用.
func SourceMap(root string, layer Layer) (map[string]string, error) {
	switch layer {
	case LayerEnv:
		return envValues(), nil
	case LayerGlobal:
		return globalValues()
	case LayerProject:
		return projectValues(root)
	case LayerSpace:
		return spaceValues(root)
	}
	return nil, fmt.Errorf("不支持的配置层 %v", layer)
}
