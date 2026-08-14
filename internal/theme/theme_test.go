// Package theme 的单元测试: 模板消息键静态校验与 dict 函数.
package theme

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// TestCheckTemplateKeys 校验 t 引用的消息键:
// 合法键通过, 缺失键报错, 变量 key 跳过, if/range 分支内同样检查.
func TestCheckTemplateKeys(t *testing.T) {
	printer := i18n.MustNew(i18n.LangEN)
	tpl := template.Must(template.New("").Funcs(funcs(printer, "")).Parse(`{{t "ThemeHomeTitle"}}
{{if .X}}{{t "ThemeArticleTag" .Page.Tag}}{{else}}{{t "ThemeFooterPowered"}}{{end}}
{{range .Items}}{{t "ThemeNavHome"}}{{end}}
{{t .DynamicKey}}
{{t "NopeMissing"}}`))
	err := checkTemplateKeys(tpl, printer)
	if err == nil {
		t.Fatal("应检测到缺失消息键 NopeMissing")
	}
	if !strings.Contains(err.Error(), "NopeMissing") {
		t.Errorf("错误应提及缺失键, got: %v", err)
	}
}

// TestCheckTemplateKeysOK 全部键合法时校验通过.
func TestCheckTemplateKeysOK(t *testing.T) {
	printer := i18n.MustNew(i18n.LangEN)
	tpl := template.Must(template.New("").Funcs(funcs(printer, "")).Parse(`{{t "ThemeHomeTitle"}} {{t "ThemeNavHome"}} {{t .Var}}`))
	if err := checkTemplateKeys(tpl, printer); err != nil {
		t.Errorf("合法模板不应报错: %v", err)
	}
}

// TestDictFunc dict 构造命名变量, t 渲染 {{ .arg0 }} 形式的消息.
func TestDictFunc(t *testing.T) {
	printer := i18n.MustNew(i18n.LangEN)
	tpl := template.Must(template.New("").Funcs(funcs(printer, "")).Parse(`{{t "ThemeArticleTag" (dict "arg0" "Go")}}`))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "Tag: Go" {
		t.Errorf("dict 命名参数渲染 = %q, want %q", got, "Tag: Go")
	}
}

// TestDictFuncOdd dict 奇数参数返回错误.
func TestDictFuncOdd(t *testing.T) {
	printer := i18n.MustNew(i18n.LangEN)
	tpl := template.Must(template.New("").Funcs(funcs(printer, "")).Parse(`{{t "ThemeHomeTitle" (dict "a")}}`))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err == nil {
		t.Error("dict 奇数参数应报错")
	}
}

// TestDictFuncBadKey dict 非字符串键返回错误.
func TestDictFuncBadKey(t *testing.T) {
	printer := i18n.MustNew(i18n.LangEN)
	tpl := template.Must(template.New("").Funcs(funcs(printer, "")).Parse(`{{t "ThemeHomeTitle" (dict 1 "x")}}`))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err == nil {
		t.Error("dict 非字符串键应报错")
	}
}

// TestLoadEmbedded 内嵌主题加载: 静态校验通过且布局齐全 (现有模板键全部存在).
func TestLoadEmbedded(t *testing.T) {
	tm, err := LoadEmbedded(i18n.LangEN, "")
	if err != nil {
		t.Fatalf("内嵌主题加载失败: %v", err)
	}
	if tm.Name != "default" {
		t.Errorf("Name = %q, want default", tm.Name)
	}
	for _, l := range []string{"home", "article", "page", "list"} {
		if !tm.HasLayout(l) {
			t.Errorf("缺少布局 %s", l)
		}
	}
}
