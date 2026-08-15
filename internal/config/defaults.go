package config

import (
	"fmt"
	"strings"
)

// validateLang 校验并规范化工具语言.
// zh 系列规范化为 zh, en 系列规范化为 en; 其他值非法.
func validateLang(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "zh", "zh-cn", "zh_hans", "zh-hans", "cn", "chinese", "中文":
		return "zh", nil
	case "en", "en-us", "en_gb", "english", "英文":
		return "en", nil
	}
	return "", fmt.Errorf("仅支持 zh / en, 收到 %q", v)
}

// Default 返回全项目配置的唯一初始化点: 所有配置项的 Key / Default / Validate 都在这里定义.
//
//	新增配置 = 加一个字段 + 在这里填一行.
func Default() *Config {
	return &Config{
		GlobalClient: GlobalClientConfig{
			Lang:  itemV("lang", "en", validateLang),
			Color: item("color", true),
		},
		GlobalServer: GlobalServerConfig{},
		SpaceClient: SpaceClientConfig{
			Lang:        itemV("lang", "en", validateLang),
			Color:       item("color", true),
			ServerToken: item("server.token", ""),
		},
		SpaceSite: SpaceSiteConfig{
			SiteTitle:       item("site.title", "我的文档"),
			SiteDescription: item("site.description", "Plainship 文档"),
			SiteURL:         item("site.url", "https://example.com"),
			SiteLanguage:    item("site.language", "en"),
			SiteID:          item("site.siteId", "my-docs"),
			BuildOutput:     item("build.output", "build"),
			ThemeName:       item("theme.name", "default"),
			ListSort:        item("list.sort", "date-desc"),
			MarkdownUnsafe:  item("markdown.unsafe", false),
			ServerURL:       item("server.url", ""),
			ServerSite:      item("server.site", "my-docs"),
		},
	}
}
