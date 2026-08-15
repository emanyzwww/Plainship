package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ConfigValue 是配置项允许的值类型.
type ConfigValue interface {
	string | bool | int | float64
}

// Item 是配置项的引擎视图.
//
// 与具体类型解耦, 供分层合并与序列化使用.
//
// 由 *ConfigItem[T] 实现.
//
// 调用方应使用类型安全的 *ConfigItem[T].
type Item interface {
	Name() string            // Name 返回配置项的唯一键名.
	SetRaw(raw string) error // SetRaw 从原始字符串解析 + 校验 + 赋值.
	Raw() string             // Raw 返回生效值的字符串形式.
	Current() any            // Current 返回当前值.
	Effective() any          // Effective 返回生效值.
	DefaultValue() any       // DefaultValue 返回默认值.
	HasValue() bool          // HasValue 判断是否显式设置过.
	Reset()                  // Reset 恢复默认值.
}

// ConfigItem 是一个类型安全的配置项.
type ConfigItem[T ConfigValue] struct {
	Key      string               // Key 是唯一键名.
	Value    T                    // Value 是显式设置的值.
	Default  T                    // Default 是默认值.
	Validate func(v T) (T, error) // Validate 是可选的校验 + 规范化函数, nil 表示不校验.
	set      bool                 // set 是否由人工设置.
}

// item 返回一个无校验的配置项.
func item[T ConfigValue](key string, def T) ConfigItem[T] {
	return ConfigItem[T]{Key: key, Default: def}
}

// itemV 返回一个带校验函数的配置项.
func itemV[T ConfigValue](key string, def T, validate func(v T) (T, error)) ConfigItem[T] {
	return ConfigItem[T]{Key: key, Default: def, Validate: validate}
}

// Get 返回当前生效值.
func (i *ConfigItem[T]) Get() T {
	if i.set {
		return i.Value
	}
	return i.Default
}

// Set 赋值并校验.
//
// 非法时返回错误且不改动 Value.
func (i *ConfigItem[T]) Set(v T) error {
	if i.Validate != nil {
		normalized, err := i.Validate(v)
		if err != nil {
			return err
		}
		v = normalized
	}
	i.Value = v
	i.set = true
	return nil
}

// Reset 恢复默认值并回到未设置状态.
func (i *ConfigItem[T]) Reset() {
	i.Value = i.Default
	i.set = false
}

// HasValue 判断是否显式设置过.
func (i *ConfigItem[T]) HasValue() bool {
	return i.set
}

// Name 返回配置项键名.
func (i *ConfigItem[T]) Name() string { return i.Key }

// Raw 返回生效值的字符串形式.
func (i *ConfigItem[T]) Raw() string { return fmt.Sprint(i.Get()) }

// Current 返回当前值.
func (i *ConfigItem[T]) Current() any { return i.Value }

// Effective 返回生效值.
func (i *ConfigItem[T]) Effective() any { return i.Get() }

// DefaultValue 返回默认值.
func (i *ConfigItem[T]) DefaultValue() any { return i.Default }

// SetRaw 从原始字符串解析 + 校验 + 赋值.
//
// 解析失败或校验失败时返回错误, Value 保持不变.
func (i *ConfigItem[T]) SetRaw(raw string) error {
	v, err := parseValue[T](raw)
	if err != nil {
		return err
	}
	return i.Set(v)
}

// parseValue 按类型解析原始字符串.
func parseValue[T ConfigValue](raw string) (T, error) {
	var zero T
	switch any(zero).(type) {
	case string:
		return any(raw).(T), nil
	case bool:
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		return any(b).(T), err
	case int:
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		return any(n).(T), err
	case float64:
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return any(f).(T), err
	}
	return zero, fmt.Errorf("unsupported config value type %T", zero)
}

// walkItems 递归遍历结构体字段, 对每个 *ConfigItem[T] 调用 fn.
//
// v 必须是可寻址的结构体值.
func walkItems(v reflect.Value, fn func(Item)) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	itemType := reflect.TypeFor[Item]()
	for _, f := range v.Fields() {
		if !f.CanAddr() {
			continue
		}
		addr := f.Addr()
		if addr.Type().Implements(itemType) {
			fn(addr.Interface().(Item))
			continue
		}
		if f.Kind() == reflect.Struct {
			walkItems(f, fn)
		}
	}
}
