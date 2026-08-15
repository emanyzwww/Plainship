package config

import (
	"fmt"
	"reflect"
)

// Layer 是配置来源层级.
type Layer int

const (
	LayerFlag    Layer = 0 // LayerFlag 命令行参数.
	LayerEnv     Layer = 1 // LayerEnv 环境变量.
	LayerSpace   Layer = 2 // LayerSpace 空间网站配置 `plainship.yaml`.
	LayerProject Layer = 3 // LayerProject 空间级客户端配置 `<space>/.plainship/config.yaml`.
	LayerGlobal  Layer = 4 // LayerGlobal 全局配置 `~/.plainship/config.yaml`.
)

// Load 读取并合并所有来源的配置, 返回生效配置与校验警告.
//
// root 为空表示不在 Space 内.
//
// flags 是命令行解析结果, 可为 nil.
//
// 合并规则:
//   - 持久化域: 每个域从自己的文件读入.
//   - 运行时层: 校验后存入 runtime.
//   - 配置文件损坏等硬错误直接返回错误.
func Load(root string, flags map[string]string) (*Config, []Warning, error) {
	cfg := Default()
	var warns []Warning

	// 1. 全局文件 -> GlobalClient / GlobalServer 域.
	gl, err := globalValues()
	if err != nil {
		return nil, nil, err
	}
	applyMap(reflect.ValueOf(&cfg.GlobalClient).Elem(), gl, &warns)
	applyMap(reflect.ValueOf(&cfg.GlobalServer).Elem(), gl, &warns)

	// 2. 空间文件 -> SpaceClient / SpaceSite 域.
	if root != "" {
		pj, err := projectValues(root)
		if err != nil {
			return nil, nil, err
		}
		applyMap(reflect.ValueOf(&cfg.SpaceClient).Elem(), pj, &warns)
		sp, err := spaceValues(root)
		if err != nil {
			return nil, nil, err
		}
		applyMap(reflect.ValueOf(&cfg.SpaceSite).Elem(), sp, &warns)
	}

	// 3. 运行时层: env 先, flag 后.
	cfg.runtime = map[string]string{}
	applyRuntime(cfg, envValues(), &warns)
	applyRuntime(cfg, flags, &warns)

	return cfg, warns, nil
}

// applyMap 把原始值 map 应用到指定域的全部配置项.
//
// 值非法时回退默认并记录警告.
func applyMap(domain reflect.Value, m map[string]string, warns *[]Warning) {
	if len(m) == 0 {
		return
	}
	walkItems(domain, func(it Item) {
		raw, ok := m[it.Name()]
		if !ok {
			return
		}
		if err := it.SetRaw(raw); err != nil {
			*warns = append(*warns, Warning{
				Key:      it.Name(),
				Given:    raw,
				Fallback: fmt.Sprint(it.DefaultValue()),
			})
			it.Reset()
		}
	})
}

// applyRuntime 把运行时值校验后存入 cfg.runtime.
//
// 非法值回退默认并记录警告.
func applyRuntime(cfg *Config, m map[string]string, warns *[]Warning) {
	for key, raw := range m {
		norm, err := runtimeValidate(key, raw)
		if err != nil {
			*warns = append(*warns, Warning{
				Key:      key,
				Given:    raw,
				Fallback: runtimeDefault(key),
			})
			cfg.runtime[key] = runtimeDefault(key)
			continue
		}
		cfg.runtime[key] = norm
	}
}

// runtimeValidate 校验运行时值并规范化.
func runtimeValidate(key, raw string) (string, error) {
	it := findItem(key)
	if it == nil {
		return raw, nil
	}
	if err := it.SetRaw(raw); err != nil {
		return "", err
	}
	return it.Raw(), nil
}

// runtimeDefault 返回指定键的默认值.
func runtimeDefault(key string) string {
	it := findItem(key)
	if it == nil {
		return ""
	}
	return fmt.Sprint(it.DefaultValue())
}

// findItem 从四个域中查找指定键的配置项.
func findItem(key string) Item {
	d := Default()
	cv := reflect.ValueOf(d).Elem()
	for _, name := range []string{"GlobalClient", "GlobalServer", "SpaceClient", "SpaceSite"} {
		var found Item
		walkItems(cv.FieldByName(name), func(it Item) {
			if found == nil && it.Name() == key {
				found = it
			}
		})
		if found != nil {
			return found
		}
	}
	return nil
}
