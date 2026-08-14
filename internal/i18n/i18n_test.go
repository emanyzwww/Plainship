package i18n

import (
	"errors"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
	}{
		{"", LangEN},
		{"zh", LangZH},
		{"zh-CN", LangZH},
		{"zh_cn", LangZH},
		{"zh-hans", LangZH},
		{"cn", LangZH},
		{"中文", LangZH},
		{"en", LangEN},
		{"en-US", LangEN},
		{"English", LangEN},
		{"fr", LangEN}, // 未知语言回退默认(en)
	}
	for _, c := range cases {
		if got := Parse(c.in); got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultLang(t *testing.T) {
	if DefaultLang() != LangEN {
		t.Error("默认语言应为 en (英文优先)")
	}
}

func TestPrinterNamedVars(t *testing.T) {
	en := MustNew(LangEN)
	// 命名变量渲染.
	if got := en.T(CliNewOk, V{"arg0": "mydoc"}); !strings.Contains(got, "Plainship Space created: mydoc") {
		t.Errorf("en named T = %q", got)
	}
	// 中文.
	zh := MustNew(LangZH)
	if got := zh.T(CliNewOk, V{"arg0": "mydoc"}); !strings.Contains(got, "已创建 Plainship Space") {
		t.Errorf("zh T = %q", got)
	}
	// 位置参数渲染 ({{ .arg0 }}).
	if got := en.T(CliNewOk, "mydoc"); !strings.Contains(got, "mydoc") {
		t.Errorf("en positional T = %q", got)
	}
	// 无变量.
	if got := en.T(CliNewNext); got != "Next steps:" {
		t.Errorf("en no-var T = %q", got)
	}
}

func TestPrinterMissingKey(t *testing.T) {
	en := MustNew(LangEN)
	if got := en.T(Key("NoSuchKey")); got != "NoSuchKey" {
		t.Errorf("missing key = %q, want key itself", got)
	}
}

func TestPrinterBareMap(t *testing.T) {
	en := MustNew(LangEN)
	// 裸 map[string]any 视为命名变量 (主题模板 dict 函数构造后传入).
	if got := en.T(CliNewOk, map[string]any{"arg0": "mydoc"}); !strings.Contains(got, "Plainship Space created: mydoc") {
		t.Errorf("bare map T = %q", got)
	}
}

func TestHas(t *testing.T) {
	en := MustNew(LangEN)
	if !en.Has(CliNewOk) {
		t.Error("Has(CliNewOk) 应为 true")
	}
	if en.Has(Key("NoSuchKey")) {
		t.Error("Has(NoSuchKey) 应为 false")
	}
	if (*L)(nil).Has(CliNewOk) {
		t.Error("nil 打印器 Has 应为 false")
	}
}

func TestLocaleConsistency(t *testing.T) {
	en := MustNew(LangEN)
	zh := MustNew(LangZH)
	for k := range en.tpls {
		if _, ok := zh.tpls[k]; !ok {
			t.Errorf("zh 缺少消息: %s", k)
		}
	}
	for k := range zh.tpls {
		if _, ok := en.tpls[k]; !ok {
			t.Errorf("en 缺少消息: %s", k)
		}
	}
}

func TestRenderVerbs(t *testing.T) {
	en := MustNew(LangEN)
	// 多参数位置渲染: arg0/arg1/arg2.
	if got := en.T(CoreBuildChanges, "docs", 1, 2, 3); !strings.Contains(got, "docs: 1 added, 2 modified, 3 deleted") {
		t.Errorf("en formatted = %q", got)
	}
	zh := MustNew(LangZH)
	if got := zh.T(CoreBuildChanges, "docs", 1, 2, 3); !strings.Contains(got, "docs: 新增 1, 修改 2, 删除 3") {
		t.Errorf("zh formatted = %q", got)
	}
}

func TestMsgError(t *testing.T) {
	// 命名变量.
	err := Errorf(CliConnectVerifyFail, V{"arg0": "boom"})
	var me *MsgError
	if !errors.As(err, &me) {
		t.Fatal("Errorf 应返回 MsgError")
	}
	if !strings.Contains(err.Error(), "connection verification failed") {
		t.Errorf("Errorf.Error() = %q", err.Error())
	}
	// 位置参数.
	err2 := Errorf(CliServeMkdirFail, "some err")
	if !strings.Contains(err2.Error(), "some err") {
		t.Errorf("positional Errorf = %q", err2.Error())
	}
	// Wrapf 保留错误链.
	base := errors.New("root cause")
	wrapped := Wrapf(base, CliServeMkdirFail)
	if !errors.Is(wrapped, base) {
		t.Error("Wrapf 应可通过 errors.Is 找到底层错误")
	}
	// RenderError.
	if got := RenderError(wrapped); !strings.Contains(got, "root cause") {
		t.Errorf("RenderError = %q", got)
	}
}

func TestRenderErrorNil(t *testing.T) {
	if got := RenderError(nil); got != "" {
		t.Errorf("RenderError(nil) = %q, want empty", got)
	}
	if got := RenderError(errors.New("plain")); got != "plain" {
		t.Errorf("RenderError(plain) = %q", got)
	}
}

func TestSetLang(t *testing.T) {
	prev := Default().Lang()
	defer SetLang(prev)
	if err := SetLang(LangZH); err != nil {
		t.Fatal(err)
	}
	if Default().Lang() != LangZH {
		t.Error("SetLang 未生效")
	}
}
