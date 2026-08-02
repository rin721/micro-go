// 本文件验证 Module 收集拒绝非法声明，并在 Register 返回后冻结 Registry。
package registration

import (
	"errors"
	"strings"
	"testing"

	"github.com/rin721/micro-go/internal/kernel/module"
)

// testModule 允许测试按场景注入名称和 Register 行为。
type testModule struct {
	// name 是 Collector 读取的模块名。
	name string
	// register 为 nil 时表示空声明模块，否则执行场景闭包。
	register func(module.Registry) error
}

// Name 返回测试配置的模块名。
func (m testModule) Name() string { return m.name }

// Register 调用可选测试闭包，并保留其错误。
func (m testModule) Register(registry module.Registry) error {
	if m.register == nil {
		return nil
	}
	return m.register(registry)
}

// TestCollectRejectsInvalidModules 表驱动覆盖 nil、空名、重名、error 和 panic。
func TestCollectRejectsInvalidModules(t *testing.T) {
	// sentinel 证明 Register 原始错误文本保留在包装结果中。
	sentinel := errors.New("register failed")
	tests := []struct {
		// name 标识子场景。
		name string
		// modules 是当前场景的完整输入序列。
		modules []module.Module
		// message 是期望稳定错误片段。
		message string
	}{
		{name: "nil", modules: []module.Module{nil}, message: "is nil"},
		{name: "empty-name", modules: []module.Module{testModule{}}, message: "empty name"},
		{name: "duplicate", modules: []module.Module{testModule{name: "same"}, testModule{name: "same"}}, message: "duplicate module name"},
		{name: "error", modules: []module.Module{testModule{name: "error", register: func(module.Registry) error { return sentinel }}}, message: sentinel.Error()},
		{name: "panic", modules: []module.Module{testModule{name: "panic", register: func(module.Registry) error { panic("boom") }}}, message: "panic: boom"},
	}
	// 每种非法输入独立调用 Collect，避免共享 Collection 状态。
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Collect(test.modules)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Collect() error = %v, want %q", err, test.message)
			}
		})
	}
}

// TestCollectFreezesRegistryAfterRegistration 证明保存 Registry 也不能延迟修改依赖图。
func TestCollectFreezesRegistryAfterRegistration(t *testing.T) {
	// 闭包故意捕获只在 Register 期间合法的 Registry 引用。
	var captured module.Registry
	_, err := Collect([]module.Module{testModule{name: "freeze", register: func(registry module.Registry) error {
		captured = registry
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	// Collect 返回后再次 Provide 必须收到 frozen 错误，且不能追加声明。
	err = module.Provide(captured, func() *struct{} { return &struct{}{} })
	if err == nil || !strings.Contains(err.Error(), "is frozen") {
		t.Fatalf("Provide() error = %v", err)
	}
}
