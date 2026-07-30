// Package idgen 定义字符串 ID 生成能力。
package idgen

type Generator interface{ New() string }
