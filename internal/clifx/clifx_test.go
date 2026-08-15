package clifx

import (
	"bytes"
	"errors"
	"os"
	"strings"
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
	if _, err := core.CreateSpace(dir); err != nil {
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

// TestSuggestFor 错误键到建议键的映射.
func TestSuggestFor(t *testing.T) {
	cases := []struct {
		key  i18n.Key
		want i18n.Key
	}{
		{i18n.ConfigNotFound, i18n.SuggestCreateSpace},
		{i18n.SpaceNotFound, i18n.SuggestCreateSpace},
		{i18n.GitNotFound, i18n.SuggestInstallGit},
		{i18n.CoreBuildNeedGit, i18n.SuggestInstallGit},
		{i18n.CorePublishNeedGit, i18n.SuggestInstallGit},
		{i18n.CorePublishNoServerURL, i18n.SuggestConnectServer},
		{i18n.SyncNoServerURL, i18n.SuggestConnectServer},
		{i18n.SyncNoServerURLSync, i18n.SuggestConnectServer},
		{i18n.CorePublishRejectDirty, i18n.SuggestBuildFirst},
		{i18n.CorePublishRejectNotBuilt, i18n.SuggestBuildFirst},
		{i18n.CorePublishRejectNoBuildDir, i18n.SuggestBuildFirst},
		{i18n.CorePublishRejectOutdated, i18n.SuggestBuildFirst},
		{i18n.CliConnectVerifyFail, i18n.SuggestCheckServer},
		{i18n.SyncConnFail, i18n.SuggestCheckServer},
		{i18n.CliTokenNotFound, i18n.SuggestServeToken},
	}
	for _, c := range cases {
		if got := SuggestFor(i18n.Errorf(c.key)); got != c.want {
			t.Errorf("SuggestFor(%s) = %q, want %q", c.key, got, c.want)
		}
	}
	// 无匹配: 普通错误 / 未知键 / nil.
	if got := SuggestFor(errors.New("plain")); got != "" {
		t.Errorf("普通错误 SuggestFor = %q, want empty", got)
	}
	if got := SuggestFor(i18n.Errorf(i18n.Key("NoSuchKey"))); got != "" {
		t.Errorf("未知键 SuggestFor = %q, want empty", got)
	}
	if got := SuggestFor(nil); got != "" {
		t.Errorf("nil SuggestFor = %q, want empty", got)
	}
	// Wrapf 包装的错误链也能命中.
	if got := SuggestFor(i18n.Wrapf(errors.New("cause"), i18n.ConfigNotFound)); got != i18n.SuggestCreateSpace {
		t.Errorf("包装错误 SuggestFor = %q, want %q", got, i18n.SuggestCreateSpace)
	}
}

// TestRenderError 错误输出包含建议行; 无建议的错误不输出.
func TestRenderError(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, i18n.Errorf(i18n.ConfigNotFound))
	out := buf.String()
	if !strings.Contains(out, "Error:") {
		t.Errorf("RenderError 应包含 Error 前缀: %s", out)
	}
	if !strings.Contains(out, "Hint:") {
		t.Errorf("RenderError 应包含建议行: %s", out)
	}
	buf.Reset()
	RenderError(&buf, i18n.Errorf(i18n.CoreSpaceExists))
	if strings.Contains(buf.String(), "Hint:") {
		t.Errorf("无建议错误不应输出 Hint: %s", buf.String())
	}
}
