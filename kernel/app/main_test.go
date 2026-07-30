// Package app_test 的 TestMain 为涉及 Runner 和监听器的测试统一执行 goroutine 泄漏检查。
package app_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain 在整包测试结束后验证框架创建的后台任务均已随 Context 和关闭流程退出。
func TestMain(m *testing.M) { goleak.VerifyTestMain(m, goleak.IgnoreCurrent()) }
