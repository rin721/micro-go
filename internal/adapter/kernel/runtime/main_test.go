// Package runtime_test 的 TestMain 为涉及 Runner 和监听器的测试统一执行 goroutine 泄漏检查。
package runtime_test

import (
	"io"
	"testing"

	configwatcher "github.com/rin721/micro-go/internal/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/internal/adapter/kernel/di/dig"
	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	app "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	kernellogging "github.com/rin721/micro-go/internal/kernel/logging"
	"go.uber.org/goleak"
)

// newRuntime 使用生产相同的默认 Adapter 装配测试 Runtime，避免被测包偷偷创建第三方实现。
func newRuntime(t *testing.T) *app.Runtime {
	t.Helper()
	return newRuntimeWithLogger(t, kernelslog.New(io.Discard))
}

// newRuntimeWithLogger 保持生产 Adapter 不变，只允许测试替换 Kernel Logger Manager。
func newRuntimeWithLogger(t *testing.T, logger kernellogging.Manager) *app.Runtime {
	// 标记为 helper 后，装配失败会定位到实际调用测试。
	t.Helper()
	// 除 Logger 可由场景替换外，其余部件与生产组合根保持一致。
	value, err := app.New(app.Dependencies{
		Logger:      logger,
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
