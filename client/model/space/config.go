package space

// Config 是站点级配置, 存储在根目录 papership.yaml.
type Config struct {
	// ==============================
	// Site Config.
	// ==============================

	SiteID          string // SiteID 站点唯一标识.
	SiteURL         string // SiteURL 站点公开基地址, 如 https://example.com.
	SiteTitle       string // SiteTitle 站点标题.
	SiteLanguage    string // SiteLanguage 默认语言, 如 zh / en.
	SiteDescription string // SiteDescription 站点描述.

	// ==============================
	// Server Config.
	// ==============================

	ServerURL  string // ServerURL 目标服务端地址.
	ServerSite string // ServerSite 服务端上的站点名, 默认使用 SiteID.

	// ==============================
	// Theme Config.
	// ==============================

	ThemeName string // ThemeName 使用的主题名, 默认 "default".
}

// LocalConfig 是客户端本地私有配置, 存储在 <space>/.papership/config.yaml.
type LocalConfig struct {
	CliLang     string // CliLang 命令行语言.
	CliColor    bool   // CliColor 是否强制启用彩色输出.
	ServerToken string // ServerToken 访问服务端的令牌.
}
