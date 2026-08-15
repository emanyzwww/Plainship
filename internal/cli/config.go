package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// configKeys 是 config 命令支持的配置键白名单.
// 键的校验与规范化由 config 包的 ConfigItem.Validate 负责 (见 config.Default).
var configKeys = map[string]bool{
	"lang": true,
}

// newConfigCmd 实现 plainship config (get / set / unset, -g 全局).
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: i18n.T(i18n.CliConfigShort),
		Long:  i18n.T(i18n.CliConfigLong),
	}

	cmd.PersistentFlags().BoolVarP(&configGlobal, "global", "g", false, i18n.T(i18n.CliConfigFlagGlobal))
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigUnsetCmd())
	return cmd
}

var configGlobal bool

// configSpaceRoot 返回项目配置所在的 Space 根目录.
func configSpaceRoot(cmd *cobra.Command) (string, error) {
	if configGlobal {
		return "", nil
	}
	return findSpaceRoot(cmd)
}

// configTarget 返回本次操作的写入目标层.
func configTarget() config.SaveTarget {
	if configGlobal {
		return config.SaveGlobal
	}
	return config.SaveProject
}

// langItem 返回本次操作目标域中的 lang 配置项.
func langItem(c *config.Config) *config.ConfigItem[string] {
	if configGlobal {
		return &c.GlobalClient.Lang
	}
	return &c.SpaceClient.Lang
}

// newConfigGetCmd 实现 plainship config get <key>.
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: i18n.T(i18n.CliConfigGetShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			key := args[0]
			if !configKeys[key] {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			root, err := configSpaceRoot(cmd)
			if err != nil {
				return err
			}
			if configGlobal {
				// 精确查看全局层: 未设置则报错.
				m, err := config.SourceMap("", config.LayerGlobal)
				if err != nil {
					return err
				}
				v, ok := m[key]
				if !ok || v == "" {
					return i18n.Errorf(i18n.CliConfigNotSet, key)
				}
				printf(out, "%s\n", v)
				return nil
			}
			// 项目: 显示生效值 (空间 > 全局 > 默认).
			c, _, err := config.Load(root, nil)
			if err != nil {
				return err
			}
			printf(out, "%s\n", langOf(c, key))
			return nil
		},
	}
}

// langOf 返回指定键的生效值.
// 当前仅 lang 一个键; 后续新增键时扩展.
func langOf(c *config.Config, key string) string {
	switch key {
	case "lang":
		return c.Lang()
	}
	return ""
}

// newConfigSetCmd 实现 plainship config set <key> <value>.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: i18n.T(i18n.CliConfigSetShort),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			key, value := args[0], args[1]
			if !configKeys[key] {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			root, err := configSpaceRoot(cmd)
			if err != nil {
				return err
			}
			// 先加载当前配置再修改: 避免覆盖文件中已有的其他配置项 (如令牌).
			c, _, err := config.Load(root, nil)
			if err != nil {
				return err
			}
			c.SetSpaceRoot(root)
			// 校验与规范化由 config 的 Validate 负责.
			if err := langItem(c).Set(value); err != nil {
				return err
			}
			if _, err := config.Save(c, configTarget()); err != nil {
				return err
			}
			scope := i18n.T(i18n.CliConfigScopeProject)
			if configGlobal {
				scope = i18n.T(i18n.CliConfigScopeGlobal)
			}
			printf(out, "%s\n", i18n.T(i18n.CliConfigSetOk, key, langItem(c).Get(), scope))
			return nil
		},
	}
}

// newConfigUnsetCmd 实现 plainship config unset <key>.
func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: i18n.T(i18n.CliConfigUnsetShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			key := args[0]
			if !configKeys[key] {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			root, err := configSpaceRoot(cmd)
			if err != nil {
				return err
			}
			// 先加载当前配置: 清空目标项, 保留其他配置项 (如令牌).
			c, _, err := config.Load(root, nil)
			if err != nil {
				return err
			}
			c.SetSpaceRoot(root)
			if key == "lang" {
				langItem(c).Reset()
			}
			if _, err := config.Save(c, configTarget()); err != nil {
				return err
			}
			scope := i18n.T(i18n.CliConfigScopeProject)
			if configGlobal {
				scope = i18n.T(i18n.CliConfigScopeGlobal)
			}
			printf(out, "%s\n", i18n.T(i18n.CliConfigUnsetOk, key, scope))
			return nil
		},
	}
}
