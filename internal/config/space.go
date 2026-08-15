package config

// SpaceClientConfig 是客户端 CLI 的空间级配置.
//
// 存储在 `<space>/.plainship/config.yaml`.
//
// 约定: 所有空间级且不应被版本控制的配置统一写在这个文件, 不为此类配置单独建文件.
type SpaceClientConfig struct {
	Lang        ConfigItem[string] // lang
	Color       ConfigItem[bool]   // color
	ServerToken ConfigItem[string] // server.token
}

// SpaceSiteConfig 是空间网站配置.
//
// 存储在 `<space>/plainship.yaml`.
type SpaceSiteConfig struct {
	SiteTitle       ConfigItem[string] // site.title
	SiteDescription ConfigItem[string] // site.description
	SiteURL         ConfigItem[string] // site.url
	SiteLanguage    ConfigItem[string] // site.language
	SiteID          ConfigItem[string] // site.siteId
	BuildOutput     ConfigItem[string] // build.output
	ThemeName       ConfigItem[string] // theme.name
	ListSort        ConfigItem[string] // list.sort
	MarkdownUnsafe  ConfigItem[bool]   // markdown.unsafe
	ServerURL       ConfigItem[string] // server.url
	ServerSite      ConfigItem[string] // server.site
}
