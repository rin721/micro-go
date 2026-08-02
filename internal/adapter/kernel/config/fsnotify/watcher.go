// Package watcher 把 fsnotify 文件事件转换为项目自有 config.Change。
// 它只负责通知“可能变化”，候选配置重建和去抖由 Application 负责。
package watcher

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rin721/micro-go/internal/kernel/config"
)

// Watcher 是 fsnotify 文件监听实现。
type Watcher struct{}

// New 创建无状态监听适配器；实际系统资源只在 Watch 调用后产生。
func New() *Watcher { return &Watcher{} }

// Watch 将实例调用转交给同包实现，便于 Bootstrap 通过接口注入 Runtime。
func (*Watcher) Watch(ctx context.Context, sources []config.Source) (<-chan config.Change, <-chan error, error) {
	return Watch(ctx, sources)
}

// Watch 启动一个由调用方 Context 管理的文件监听器。
// 返回带缓冲通道并采用非阻塞发送，文件事件风暴不会反压 fsnotify；丢失的重复通知
// 没有问题，因为收到任意一次通知都会触发后续全量重建。
func Watch(ctx context.Context, sources []config.Source) (<-chan config.Change, <-chan error, error) {
	// 先创建唯一的底层 watcher；后续任何注册失败都会在返回前关闭它。
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("create file watcher: %w", err)
	}
	// 监听父目录而不是只监听文件，才能覆盖编辑器通过临时文件 rename 替换原文件的保存方式。
	// names 同时承担精确文件过滤和从绝对路径还原 Source 名称的职责。
	names := make(map[string]string)
	// 普通 Source 没有监听契约，会保留在 Reload 全量加载列表中但不注册文件事件。
	for _, source := range sources {
		watchable, ok := source.(config.WatchSource)
		if !ok {
			continue
		}
		// 清理路径表示法，确保注册键与 fsnotify 回传路径使用同一比较形式。
		path := filepath.Clean(watchable.WatchDescriptor().Path)
		names[path] = source.Name()
		if err := watch.Add(filepath.Dir(path)); err != nil {
			// 同时保留注册错误和 watcher 关闭错误，不能因清理失败覆盖主原因。
			return nil, nil, errors.Join(fmt.Errorf("watch config directory for %q: %w", path, err), watch.Close())
		}
	}
	// 容量 1 用于合并尚未消费的重复事件，而不是缓存每一次文件系统抖动。
	events := make(chan config.Change, 1)
	errorsChannel := make(chan error, 1)
	// 只有明确启用 Watch 且存在 WatchSource 时上层才调用本函数，因此空载应用没有后台 goroutine。
	go func() {
		// 生产者退出后关闭只读通道，让 Application 可以确定监听生命周期已经结束。
		defer close(events)
		defer close(errorsChannel)
		// goroutine 是底层 watcher 的唯一运行期所有者，退出时统一释放句柄。
		defer watch.Close()
		// 单循环串行处理取消、错误和文件事件，避免为每个事件继续派生 goroutine。
		for {
			select {
			case <-ctx.Done():
				// Context 取消是正常停止信号，错误由上层根 Context 语义决定。
				return
			case err, ok := <-watch.Errors:
				// 底层通道关闭表示 watcher 已结束，无需再等待事件通道。
				if !ok {
					return
				}
				// 错误通道满时丢弃重复错误，防止文件系统生产者被观察方反压。
				select {
				case errorsChannel <- fmt.Errorf("watch configuration: %w", err):
				default:
				}
			case event, ok := <-watch.Events:
				// 事件通道关闭后结束生产者并由 defer 关闭项目通道。
				if !ok {
					return
				}
				// 父目录中其他文件的事件不属于已声明配置源，直接忽略。
				path := filepath.Clean(event.Name)
				name, tracked := names[path]
				if !tracked {
					continue
				}
				// chmod、remove 等事件不会提供可立即重载的新内容，等待后续 create/write/rename。
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				// 非阻塞发送把事件风暴压缩为“至少发生过一次变化”的事实。
				select {
				case events <- config.Change{Source: name, At: time.Now().UTC()}:
				default:
				}
			}
		}
	}()
	// 通道所有权交给调用方读取；只有内部 goroutine 会发送和关闭。
	return events, errorsChannel, nil
}
