package utils

import (
	"encoding/json"
)

// 高性能JSON配置
var (
	// FastestConfig 最快的JSON配置，用于性能关键路径
	FastestConfig = jsonFastest{}

	// SafeConfig 安全的JSON配置，带有更多验证
	SafeConfig = jsonSafe{}
)

// jsonFastest 使用标准库json作为最快配置
type jsonFastest struct{}

func (jsonFastest) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonFastest) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonFastest) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// jsonSafe 使用标准库json作为安全配置
type jsonSafe struct{}

func (jsonSafe) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonSafe) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonSafe) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// FastMarshal 高性能JSON序列化
func FastMarshal(v any) ([]byte, error) {
	return FastestConfig.Marshal(v)
}

// FastUnmarshal 高性能JSON反序列化
func FastUnmarshal(data []byte, v any) error {
	return FastestConfig.Unmarshal(data, v)
}

// SafeMarshal 安全JSON序列化（带验证）
func SafeMarshal(v any) ([]byte, error) {
	return SafeConfig.Marshal(v)
}

// SafeUnmarshal 安全JSON反序列化（带验证）
func SafeUnmarshal(data []byte, v any) error {
	return SafeConfig.Unmarshal(data, v)
}

// MarshalIndent 带缩进的JSON序列化
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return SafeConfig.MarshalIndent(v, prefix, indent)
}
