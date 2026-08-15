package clifx

import (
	"os"
	"testing"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// TestDetectLang 验证语言链: 环境变量 > 项目配置 > 全局配置 > 默认.
func TestDetectLang(t *testing.T) {
	home := t.TempDir()
	// 隔离用户主目录: Windows 用 USERPROFILE, Linux/macOS 用 HOME.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := t.TempDir()
	if _, err := core.CreateSpace(dir, nil); err != nil {
		t.Fatalf("CreateSpace 失败: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// 默认 en.
	if got := DetectLang(); got != i18n.LangEN {
		t.Errorf("默认 DetectLang = %v, 期望 en", got)
	}
	// 全局 zh.
	c := config.Default()
	if err := c.GlobalClient.Lang.Set("zh"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Save(c, config.SaveGlobal); err != nil {
		t.Fatal(err)
	}
	if got := DetectLang(); got != i18n.LangZH {
		t.Errorf("全局 zh 后 DetectLang = %v", got)
	}
	// 项目 en 覆盖全局 zh.
	c2 := config.Default()
	c2.SetSpaceRoot(dir)
	if err := c2.SpaceClient.Lang.Set("en"); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Save(c2, config.SaveProject); err != nil {
		t.Fatal(err)
	}
	if got := DetectLang(); got != i18n.LangEN {
		t.Errorf("项目 en 应覆盖全局, DetectLang = %v", got)
	}
	// 环境变量覆盖配置.
	t.Setenv("PLAINSHIP_LANG", "zh")
	if got := DetectLang(); got != i18n.LangZH {
		t.Errorf("环境变量应覆盖配置, DetectLang = %v", got)
	}
}

// TestApplyLangEarly --lang 参数优先于环境变量, 且支持 --lang=zh 形式.
func TestApplyLangEarly(t *testing.T) {
	t.Setenv("PLAINSHIP_LANG", "en")
	ApplyLangEarly([]string{"--lang", "zh", "status"})
	if got := i18n.Default().Lang(); got != i18n.LangZH {
		t.Errorf("--lang zh 后 Default().Lang() = %v", got)
	}
	i18n.SetLang(i18n.LangEN)
	ApplyLangEarly([]string{"--lang=zh", "status"})
	if got := i18n.Default().Lang(); got != i18n.LangZH {
		t.Errorf("--lang=zh 后 Default().Lang() = %v", got)
	}
	i18n.SetLang(i18n.LangEN)
	ApplyLangEarly([]string{"status", "--lang", "zh"})
	if got := i18n.Default().Lang(); got != i18n.LangZH {
		t.Errorf("参数末尾 --lang zh 后 Default().Lang() = %v", got)
	}
	i18n.SetLang(i18n.LangEN)
	ApplyLangEarly([]string{"status"})
	if got := i18n.Default().Lang(); got != i18n.LangEN {
		t.Errorf("无 --lang 时应保持默认 en, 实际 %v", got)
	}
}
