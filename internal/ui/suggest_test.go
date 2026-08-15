package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// TestSuggestFor 错误键到建议键的映射, 自 clifx 迁移.
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

// TestRenderError 错误输出包含建议行, 无建议的错误不输出, 自 clifx 迁移.
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
