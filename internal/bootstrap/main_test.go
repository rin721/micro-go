package bootstrap

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain 验证 Bootstrap 装配的 Runner 与配置监听任务都会随关闭流程退出。
func TestMain(m *testing.M) { goleak.VerifyTestMain(m, goleak.IgnoreCurrent()) }
