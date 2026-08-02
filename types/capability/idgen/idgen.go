// Package idgen 定义字符串 ID 生成能力。
package idgen

// Generator 生成项目统一使用的字符串 ID。
// 返回 string 而不是第三方 UUID 类型，使业务模型可以在不导入 UUID 库的情况下更换实现。
type Generator interface {
	// New 每次调用生成一个新的字符串标识。
	New() string
}
