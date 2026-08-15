package config

// Warning 是一条配置校验警告.
//
// 语义: 某个来源提供了非法值, 该值被忽略, 配置项回退到默认值.
type Warning struct {
	Key      string // Key 是配置项键名.
	Given    string // Given 是来源提供的原始值.
	Fallback string // Fallback 是最终生效的值.
}
