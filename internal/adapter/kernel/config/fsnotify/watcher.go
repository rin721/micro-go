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
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("create file watcher: %w", err)
	}
	// 监听父目录而不是只监听文件，才能覆盖编辑器通过临时文件 rename 替换原文件的保存方式。
	names := make(map[string]string)
	for _, source := range sources {
		watchable, ok := source.(config.WatchSource)
		if !ok {
			continue
		}
		path := filepath.Clean(watchable.WatchDescriptor().Path)
		names[path] = source.Name()
		if err := watch.Add(filepath.Dir(path)); err != nil {
			return nil, nil, errors.Join(fmt.Errorf("watch config directory for %q: %w", path, err), watch.Close())
		}
	}
	events := make(chan config.Change, 1)
	errorsChannel := make(chan error, 1)
	// 只有明确启用 Watch 且存在 WatchSource 时上层才调用本函数，因此空载应用没有后台 goroutine。
	go func() {
		defer close(events)
		defer close(errorsChannel)
		defer watch.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-watch.Errors:
				if !ok {
					return
				}
				select {
				case errorsChannel <- fmt.Errorf("watch configuration: %w", err):
				default:
				}
			case event, ok := <-watch.Events:
				if !ok {
					return
				}
				path := filepath.Clean(event.Name)
				name, tracked := names[path]
				if !tracked {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				select {
				case events <- config.Change{Source: name, At: time.Now().UTC()}:
				default:
				}
			}
		}
	}()
	return events, errorsChannel, nil
}
