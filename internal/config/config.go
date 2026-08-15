package config

import (
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/emanyzwww/plainship/internal/hash"
)

// Config 是全项目唯一的配置对象, 分为四个域:
//   - GlobalClient: 客户端 CLI 全局配置 `~/.plainship/config.yaml`.
//   - GlobalServer: 服务端 CLI 全局配置 `~/.plainship/config.yaml`.
//   - SpaceClient:  客户端 CLI 空间配置 `<space>/.plainship/config.yaml`.
//   - SpaceSite:    空间网站配置 `<space>/plainship.yaml`.
//
// 生效值解析: `flag` > `env` > Space 域 > Global 域 > 默认值.
//
// flag/env 属于运行时层, 只影响本次运行, 永不落盘.
type Config struct {
	GlobalClient GlobalClientConfig
	GlobalServer GlobalServerConfig
	SpaceClient  SpaceClientConfig
	SpaceSite    SpaceSiteConfig
	spaceRoot    string
	runtime      map[string]string
}

// SetSpaceRoot 记录 Space 根目录.
func (c *Config) SetSpaceRoot(root string) {
	c.spaceRoot = root
}

// Lang 返回客户端 CLI 工具语言的生效值.
func (c *Config) Lang() string {
	if v, ok := c.runtime["lang"]; ok {
		return v
	}
	if c.SpaceClient.Lang.HasValue() {
		return c.SpaceClient.Lang.Get()
	}
	return c.GlobalClient.Lang.Get()
}

// Color 返回客户端 CLI 彩色输出的生效值.
func (c *Config) Color() bool {
	if v, ok := c.runtime["color"]; ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
		return c.GlobalClient.Color.Get()
	}
	if c.SpaceClient.Color.HasValue() {
		return c.SpaceClient.Color.Get()
	}
	return c.GlobalClient.Color.Get()
}

// ServerToken 返回发布访问令牌的生效值.
func (c *Config) ServerToken() string {
	return c.SpaceClient.ServerToken.Get()
}

// Hash 计算空间网站配置的规范化哈希, 用于变化检测.
func (c *Config) Hash() (string, error) {
	m := map[string]any{}
	site := reflect.ValueOf(c).Elem().FieldByName("SpaceSite")
	walkItems(site, func(it Item) {
		setNested(m, it.Name(), it.Effective())
	})
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return hash.Bytes(data), nil
}
