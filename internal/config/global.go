package config

// GlobalClientConfig 是客户端 CLI 的全局配置.
//
// 存储在 `~/.plainship/config.yaml`.
type GlobalClientConfig struct {
	Lang  ConfigItem[string] // lang
	Color ConfigItem[bool]   // color
}

// GlobalServerConfig 是服务端 CLI 的全局配置.
//
// 存储在 `~/.plainship/config.yaml`.
type GlobalServerConfig struct{}
