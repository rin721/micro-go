// 本文件验证 fsnotify Adapter 的目录失败、取消收尾和原子替换事件语义。
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/kernel/config"
)

// TestWatchRejectsMissingDirectory 确保监听注册失败时不返回任何可误用通道。
func TestWatchRejectsMissingDirectory(t *testing.T) {
	// Source 可以指向尚不存在的文件，但其父目录同样不存在会让 watcher.Add 失败。
	source, err := configsource.FromFile(filepath.Join(t.TempDir(), "missing", "app.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// 三个返回值必须形成原子失败：两个通道均为 nil，错误非 nil。
	events, failures, err := Watch(context.Background(), []config.Source{source})
	if err == nil || events != nil || failures != nil {
		t.Fatalf("Watch() = (%v, %v, %v)", events, failures, err)
	}
}

// TestWatchClosesChannelsAfterCancellation 验证 Context 取消会终止生产者并关闭两条通道。
func TestWatchClosesChannelsAfterCancellation(t *testing.T) {
	// 创建真实文件和 Source，确保测试进入已成功启动 watcher 的路径。
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("app: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, failures, err := Watch(ctx, []config.Source{source})
	if err != nil {
		t.Fatal(err)
	}
	// 取消是唯一退出信号，随后等待事件与错误通道分别被生产者关闭。
	cancel()
	deadline := time.After(time.Second)
	// 通道关闭后置 nil，循环直到两者都确认关闭；deadline 防止测试永久挂起。
	for events != nil || failures != nil {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case _, ok := <-failures:
			if !ok {
				failures = nil
			}
		case <-deadline:
			t.Fatal("watch channels did not close after cancellation")
		}
	}
}

// TestWatchReportsAtomicFileReplacement 验证监听父目录能够捕获编辑器 rename 保存方式。
func TestWatchReportsAtomicFileReplacement(t *testing.T) {
	// 真实临时目录避免平台文件事件与仓库文件互相干扰。
	directory := t.TempDir()
	path := filepath.Join(directory, "app.yaml")
	if err := os.WriteFile(path, []byte("app: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, failures, err := Watch(ctx, []config.Source{source})
	if err != nil {
		t.Fatal(err)
	}
	// 把原文件重命名成备份模拟原子替换的第一步，应产生 Rename 事件。
	if err := os.Rename(path, filepath.Join(directory, "app.previous.yaml")); err != nil {
		t.Fatal(err)
	}
	// 事件、Watcher 错误和超时三者竞争，确保测试失败不会静默等待。
	select {
	case <-events:
	case err := <-failures:
		t.Fatalf("watch failure: %v", err)
	case <-time.After(time.Second):
		t.Fatal("atomic replacement did not produce a file event")
	}
}
