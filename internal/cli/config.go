package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/cliconfig"
	"github.com/emanyzwww/Plainship/internal/i18n"
)

// configKey 是配置项字段访问器.
type configKey struct {
	// get 返回字段指针, 用于读改配置项.
	get func(cfg *cliconfig.Config) *string

	// validate 校验新值, 返回规范化后的值; 非法时返回错误.
	validate func(value string) (string, error)
}

// configKeys 是支持的配置项白名单 (当前仅 lang).
var configKeys = map[string]configKey{
	"lang": {
		get: func(cfg *cliconfig.Config) *string { return &cfg.Lang },
		validate: func(value string) (string, error) {
			// 严格匹配: Parse 会宽容回退默认语言, 此处必须拒绝非法值.
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "zh", "zh-cn", "zh_hans", "zh-hans", "cn", "chinese", "中文":
				return "zh", nil
			case "en", "en-us", "en_gb", "english", "英文":
				return "en", nil
			default:
				return "", i18n.Errorf(i18n.CliConfigInvalidLang, value)
			}
		},
	},
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

// configLoad 加载目标层配置 (全局或项目).
func configLoad(spaceRoot string) (cliconfig.Config, error) {
	if configGlobal {
		return cliconfig.LoadGlobal()
	}
	return cliconfig.LoadProject(spaceRoot)
}

// configSave 保存目标层配置.
func configSave(spaceRoot string, cfg cliconfig.Config) error {
	if configGlobal {
		return cliconfig.SaveGlobal(cfg)
	}
	return cliconfig.SaveProject(spaceRoot, cfg)
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
			ck, ok := configKeys[key]
			if !ok {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			if configGlobal {
				cfg, err := cliconfig.LoadGlobal()
				if err != nil {
					return err
				}
				v := *ck.get(&cfg)
				if v == "" {
					return i18n.Errorf(i18n.CliConfigNotSet, key)
				}
				printf(out, "%s\n", v)
				return nil
			}
			// 项目: 显示生效值 (项目 > 全局 > 默认).
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			cfg := cliconfig.LoadEffective(root)
			v := *ck.get(&cfg)
			if v == "" {
				v = i18n.T(i18n.CliConfigDefaultLang)
			}
			printf(out, "%s\n", v)
			return nil
		},
	}
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
			ck, ok := configKeys[key]
			if !ok {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			normalized, err := ck.validate(value)
			if err != nil {
				return err
			}
			root, err := configSpaceRoot(cmd)
			if err != nil {
				return err
			}
			cfg, err := configLoad(root)
			if err != nil {
				return err
			}
			*ck.get(&cfg) = normalized
			if err := configSave(root, cfg); err != nil {
				return err
			}
			scope := i18n.T(i18n.CliConfigScopeProject)
			if configGlobal {
				scope = i18n.T(i18n.CliConfigScopeGlobal)
			}
			printf(out, "%s\n", i18n.T(i18n.CliConfigSetOk, key, normalized, scope))
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
			ck, ok := configKeys[key]
			if !ok {
				return i18n.Errorf(i18n.CliConfigInvalidKey, key)
			}
			root, err := configSpaceRoot(cmd)
			if err != nil {
				return err
			}
			cfg, err := configLoad(root)
			if err != nil {
				return err
			}
			*ck.get(&cfg) = ""
			if err := configSave(root, cfg); err != nil {
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
