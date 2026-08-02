// 本文件用最小声明图验证 Provider 签名、双层循环检测和确定性图输出。
package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/rin721/micro-go/internal/adapter/kernel/module"
	publicmodule "github.com/rin721/micro-go/internal/kernel/module"
)

// declaration 为测试构造带模块归属的最小 Provider 声明。
func declaration(owner string, order int, constructor any) registration.Provider {
	return registration.Provider{Module: owner, ModuleOrder: order, Declaration: publicmodule.ProviderDeclaration{Constructor: constructor}}
}

// TestCompileRejectsInvalidProviderSignatures 表驱动覆盖 nil、非函数、变参和非法返回值。
func TestCompileRejectsInvalidProviderSignatures(t *testing.T) {
	// tests 的 message 只匹配稳定项目错误片段，不绑定反射错误的全部格式。
	tests := []struct {
		// name 标识具体无效签名场景。
		name string
		// constructor 是交给 Compiler 检查的任意声明值。
		constructor any
		// message 是期望错误包含的规则说明。
		message string
	}{
		{name: "nil", constructor: nil, message: "nil provider"},
		{name: "not-function", constructor: 42, message: "must be a function"},
		{name: "variadic", constructor: func(...string) *struct{} { return nil }, message: "must not be variadic"},
		{name: "no-result", constructor: func() {}, message: "must return Component"},
		{name: "interface-result", constructor: func() error { return nil }, message: "must return a concrete component"},
	}
	// 每种签名独立运行，便于精确定位规则回归。
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(registration.Collection{Providers: []registration.Provider{declaration("test", 0, test.constructor)}})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Compile() error = %v, want %q", err, test.message)
			}
		})
	}
}

// providerCycleA 直接依赖 B，用于建立 Provider 环的一侧。
type providerCycleA struct {
	// b 是形成环的直接具体依赖。
	b *providerCycleB
}

// providerCycleB 反向依赖 A，闭合 Provider 环。
type providerCycleB struct {
	// a 是形成环的直接具体依赖。
	a *providerCycleA
}

// TestCompileRejectsProviderCycle 验证具体 Provider 循环在调用 Dig 前失败。
func TestCompileRejectsProviderCycle(t *testing.T) {
	// 两个构造函数结果和参数互指，无法生成拓扑顺序。
	collection := registration.Collection{Providers: []registration.Provider{
		declaration("cycle", 0, func(b *providerCycleB) *providerCycleA { return &providerCycleA{b: b} }),
		declaration("cycle", 0, func(a *providerCycleA) *providerCycleB { return &providerCycleB{a: a} }),
	}}
	_, err := Compile(collection)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("Compile() error = %v", err)
	}
}

// moduleContractA 是 module-a 对外公开的最小接口。
type moduleContractA interface {
	// A 标记 A 契约的方法集。
	A()
}

// moduleContractB 是 module-b 对外公开的最小接口。
type moduleContractB interface {
	// B 标记 B 契约的方法集。
	B()
}

// moduleImplementationA 是 A 契约的无状态实现。
type moduleImplementationA struct{}

// moduleImplementationB 是 B 契约的无状态实现。
type moduleImplementationB struct{}

// moduleConsumerA 位于 module-a，但依赖 module-b 的公开接口。
type moduleConsumerA struct {
	// dependency 建立从 module-a 到 module-b 的模块边。
	dependency moduleContractB
}

// moduleConsumerB 位于 module-b，但反向依赖 module-a 的公开接口。
type moduleConsumerB struct {
	// dependency 建立从 module-b 到 module-a 的模块边。
	dependency moduleContractA
}

// A 让 moduleImplementationA 满足测试契约。
func (*moduleImplementationA) A() {}

// B 让 moduleImplementationB 满足测试契约。
func (*moduleImplementationB) B() {}

// TestCompileRejectsModuleCycleWithoutProviderCycle 验证 Provider 无环时仍拒绝模块职责环。
func TestCompileRejectsModuleCycleWithoutProviderCycle(t *testing.T) {
	// 先提取接口和实现的准确反射类型供 Binding 使用。
	contractA := reflect.TypeOf((*moduleContractA)(nil)).Elem()
	contractB := reflect.TypeOf((*moduleContractB)(nil)).Elem()
	implementationA := reflect.TypeOf((*moduleImplementationA)(nil))
	implementationB := reflect.TypeOf((*moduleImplementationB)(nil))
	collection := registration.Collection{
		// 每个具体 Provider 图都可排序，但两个 Consumer 形成跨模块双向依赖。
		Providers: []registration.Provider{
			declaration("module-a", 0, func() *moduleImplementationA { return &moduleImplementationA{} }),
			declaration("module-a", 0, func(value moduleContractB) *moduleConsumerA { return &moduleConsumerA{dependency: value} }),
			declaration("module-b", 1, func() *moduleImplementationB { return &moduleImplementationB{} }),
			declaration("module-b", 1, func(value moduleContractA) *moduleConsumerB { return &moduleConsumerB{dependency: value} }),
		},
		Bindings: []registration.Binding{
			{Module: "module-a", Declaration: publicmodule.BindingDeclaration{Contract: contractA, Implementation: implementationA}},
			{Module: "module-b", ModuleOrder: 1, Declaration: publicmodule.BindingDeclaration{Contract: contractB, Implementation: implementationB}},
		},
		Exports: []registration.Export{
			{Module: "module-a", Declaration: publicmodule.ExportDeclaration{Contract: contractA}},
			{Module: "module-b", ModuleOrder: 1, Declaration: publicmodule.ExportDeclaration{Contract: contractB}},
		},
	}
	_, err := Compile(collection)
	if err == nil || !strings.Contains(err.Error(), "module dependency cycle detected among module-a, module-b") {
		t.Fatalf("Compile() error = %v", err)
	}
}

// stableA 是确定性图测试的根依赖。
type stableA struct{}

// stableB 是只依赖 stableA 的消费者。
type stableB struct {
	// a 建立唯一依赖边。
	a *stableA
}

// TestCompileProducesStableGraph 验证相同声明重复编译产生逐字节一致的 JSON 图。
func TestCompileProducesStableGraph(t *testing.T) {
	// 简单无环图仍包含多个节点，足以检查节点和边顺序。
	collection := registration.Collection{Providers: []registration.Provider{
		declaration("stable", 0, func() *stableA { return &stableA{} }),
		declaration("stable", 0, func(a *stableA) *stableB { return &stableB{a: a} }),
	}}
	first, err := Compile(collection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(collection)
	if err != nil {
		t.Fatal(err)
	}
	// 使用公共稳定 JSON 投影比较，而不是依赖内部指针或 map 顺序。
	firstJSON, err := first.Graph.JSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.Graph.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("graph is not stable:\n%s\n%s", firstJSON, secondJSON)
	}
}
