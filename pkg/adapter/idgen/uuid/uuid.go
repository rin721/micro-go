// Package uuid 使用 Google UUID 实现项目 ID 契约。
package uuid

import (
	googleuuid "github.com/google/uuid"
	"github.com/rin721/micro-go/types/capability/idgen"
)

// Generator 在适配层内部使用 google/uuid，向外只返回项目约定的 string。
type Generator struct{}

// New 创建无状态 UUID 生成器。
func New() *Generator { return &Generator{} }

// New 生成标准 UUID 字符串，第三方 UUID 类型不会进入业务模型。
func (*Generator) New() string { return googleuuid.NewString() }

// 编译期断言防止 Adapter 与公共契约漂移。
var _ idgen.Generator = (*Generator)(nil)
