// 本文件验证构造 Context 取消时 Dig Adapter 会把已完成实例所有权交还 Runtime。
package digadapter

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
)

// cancelCloseable 记录自己是否被关闭，用于检查 Adapter 不越权清理资源。
type cancelCloseable struct {
	// closed 由 Close 设置，初始 false 表示资源所有权尚未释放。
	closed bool
}

// Close 标记测试资源已被其真正所有者关闭。
func (c *cancelCloseable) Close(context.Context) error {
	c.closed = true
	return nil
}

// afterCancellation 是取消发生后不应再被构造的消费者。
type afterCancellation struct {
	// dependency 指向第一个已完成实例。
	dependency *cancelCloseable
}

// TestConstructCancellationReturnsCompletedInstancesToRuntime 锁定取消检查与清理所有权边界。
func TestConstructCancellationReturnsCompletedInstancesToRuntime(t *testing.T) {
	// 第一个 Provider 构造成功后立即取消，制造两个 Provider 之间的取消窗口。
	ctx, cancel := context.WithCancel(context.Background())
	var first *cancelCloseable
	firstConstructor := func() *cancelCloseable {
		first = &cancelCloseable{}
		cancel()
		return first
	}
	secondConstructor := func(value *cancelCloseable) *afterCancellation {
		return &afterCancellation{dependency: value}
	}
	// 冻结 Plan 明确第一个实例在前、消费者在后。
	plan := &compiled.Plan{Providers: []compiled.Provider{
		{ID: "test:first", Module: "test", Name: "first", Type: reflect.TypeOf((*cancelCloseable)(nil)), Constructor: reflect.ValueOf(firstConstructor)},
		{ID: "test:second", Module: "test", Name: "second", Type: reflect.TypeOf((*afterCancellation)(nil)), Constructor: reflect.ValueOf(secondConstructor), Dependencies: []compiled.Dependency{{Requested: reflect.TypeOf((*cancelCloseable)(nil)), Resolved: reflect.TypeOf((*cancelCloseable)(nil))}}},
	}}
	instances, err := New().Construct(ctx, plan, nil)
	// Adapter 必须返回一个已完成实例和 context.Canceled，不能丢失回滚清单。
	if len(instances) != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Construct() = (%+v, %v)", instances, err)
	}
	// Dig Adapter 不负责 Close；Runtime 会使用独立清理 Context 统一回滚。
	if first == nil || first.closed {
		t.Fatal("constructor engine must return ownership without closing the instance")
	}
}
