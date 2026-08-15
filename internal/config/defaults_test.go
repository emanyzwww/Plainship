package config

import (
	"reflect"
	"testing"
)

// TestDefault_AllItemsValid 每个配置项必须: 键名非空, 且默认值通过自身校验.
func TestDefault_AllItemsValid(t *testing.T) {
	cfg := Default()
	walkItems(reflect.ValueOf(cfg).Elem(), func(it Item) {
		if it.Name() == "" {
			t.Error("存在空键名的配置项")
		}
		// 默认值必须合法: 用默认值的字符串形式调用 SetRaw 应成功.
		if err := it.SetRaw(it.Raw()); err != nil {
			t.Errorf("配置项 %s 的默认值不合法: %v", it.Name(), err)
		}
	})
}

// TestDefault_NewInstanceEachTime Default 每次返回全新实例, 互不影响.
func TestDefault_NewInstanceEachTime(t *testing.T) {
	a := Default()
	b := Default()
	_ = a.GlobalClient.Lang.Set("zh")
	if b.Lang() != "en" {
		t.Error("Default 应返回全新实例")
	}
}

// TestEffective_Methods 生效方法: 空间覆盖全局, 运行时覆盖一切.
func TestEffective_Methods(t *testing.T) {
	cfg := Default()
	// 空间覆盖全局.
	_ = cfg.GlobalClient.Lang.Set("zh")
	_ = cfg.SpaceClient.Lang.Set("en")
	if cfg.Lang() != "en" {
		t.Errorf("空间应覆盖全局: %q", cfg.Lang())
	}
	// 运行时覆盖空间.
	cfg.runtime = map[string]string{"lang": "zh"}
	if cfg.Lang() != "zh" {
		t.Errorf("运行时应覆盖空间: %q", cfg.Lang())
	}
	// color 正向语义.
	if !cfg.Color() {
		t.Error("color 默认应为 true")
	}
	_ = cfg.SpaceClient.Color.Set(false)
	if cfg.Color() {
		t.Error("color=false 应生效")
	}
}
