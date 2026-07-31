// Package runtime_test 的 TestMain 为涉及 Runner 和监听器的测试统一执行 goroutine 泄漏检查。
package runtime_test

import (
	"testing"

	configwatcher "github.com/rin721/micro-go/pkg/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/pkg/adapter/kernel/config/koanf"
	"github.com/rin721/micro-go/pkg/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/pkg/adapter/kernel/di/dig"
	registration "github.com/rin721/micro-go/pkg/adapter/kernel/module"
	app "github.com/rin721/micro-go/pkg/adapter/kernel/runtime"
	"go.uber.org/goleak"
)

// newRuntime 使用生产相同的默认 Adapter 装配测试 Runtime，避免被测包偷偷创建第三方实现。
func newRuntime(t *testing.T) *app.Runtime {
	t.Helper()
	value, err := app.New(app.Dependencies{
		Collector:   registration.NewCollector(),
		Compiler:    compiler.New(),
		Loader:      koanfadapter.New(),
		Constructor: digadapter.New(),
		Watcher:     configwatcher.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// TestMain 在整包测试结束后验证 Runtime 创建的后台任务均已随 Context 和关闭流程退出。
func TestMain(m *testing.M) { goleak.VerifyTestMain(m, goleak.IgnoreCurrent()) }
