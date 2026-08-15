package config

import (
	"testing"
)

func TestItem_GetSetReset(t *testing.T) {
	it := item("lang", "en")
	if it.Get() != "en" {
		t.Errorf("默认值 = %q, 期望 en", it.Get())
	}
	if it.HasValue() {
		t.Error("未设置时 HasValue 应为 false")
	}
	if err := it.Set("zh"); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	if it.Get() != "zh" || !it.HasValue() {
		t.Errorf("Set 后 = %q, HasValue=%v", it.Get(), it.HasValue())
	}
	it.Reset()
	if it.Get() != "en" || it.HasValue() {
		t.Errorf("Reset 后 = %q, HasValue=%v", it.Get(), it.HasValue())
	}
}

func TestItem_SetRaw_Parse(t *testing.T) {
	var s ConfigItem[string]
	if err := s.SetRaw("abc"); err != nil || s.Get() != "abc" {
		t.Errorf("string 解析失败: %v %q", err, s.Get())
	}
	var b ConfigItem[bool]
	if err := b.SetRaw("true"); err != nil || !b.Get() {
		t.Errorf("bool true 解析失败: %v", err)
	}
	if err := b.SetRaw("0"); err != nil || b.Get() {
		t.Errorf("bool 0 解析失败: %v", err)
	}
	if err := b.SetRaw("notabool"); err == nil {
		t.Error("非法 bool 应报错")
	}
	var n ConfigItem[int]
	if err := n.SetRaw("42"); err != nil || n.Get() != 42 {
		t.Errorf("int 解析失败: %v", err)
	}
	if err := n.SetRaw("x"); err == nil {
		t.Error("非法 int 应报错")
	}
	var f ConfigItem[float64]
	if err := f.SetRaw("3.14"); err != nil || f.Get() != 3.14 {
		t.Errorf("float64 解析失败: %v", err)
	}
}

func TestItem_Validate(t *testing.T) {
	it := itemV("lang", "en", func(v string) (string, error) {
		if v == "zh" || v == "en" {
			return v, nil
		}
		return "", errTestInvalid
	})
	if err := it.Set("zh-CN"); err == nil {
		t.Error("非法值应被校验拦截")
	}
	if it.Get() != "en" {
		t.Errorf("校验失败后 Value 不应被改动, 实际 %q", it.Get())
	}
	if err := it.SetRaw("zh"); err != nil || it.Get() != "zh" {
		t.Errorf("合法值 SetRaw 失败: %v", err)
	}
}

var errTestInvalid = &testError{}

type testError struct{}

func (e *testError) Error() string { return "invalid" }
