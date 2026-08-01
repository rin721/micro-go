// Package workitems 定义后端纵切片验收使用的业务模型、消费者契约和应用服务。
package workitems

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rin721/micro-go/types/capability/clock"
	"github.com/rin721/micro-go/types/capability/idgen"
	"github.com/rin721/micro-go/types/capability/logging"
)

// MaxTitleLength 是 Work Item 标题允许的最大 Unicode 字符数。
const MaxTitleLength = 200

// Status 是 Work Item 的有限生命周期状态。
type Status string

const (
	// StatusOpen 表示工作项尚未完成。
	StatusOpen Status = "open"
	// StatusCompleted 表示工作项已经完成，CompletedAt 必须存在。
	StatusCompleted Status = "completed"
)

var (
	// ErrNotFound 表示指定 Work Item 不存在。
	ErrNotFound = errors.New("work item not found")
	// ErrInvalidTitle 表示标题为空或超过 MaxTitleLength。
	ErrInvalidTitle = errors.New("work item title is invalid")
)

// Item 是 Work Item 纵切片跨应用与传输边界使用的业务事实。
type Item struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Repository 由应用服务按业务需要定义，持久化 Adapter 依赖并实现该契约。
type Repository interface {
	Create(context.Context, Item) error
	Get(context.Context, string) (Item, error)
	Complete(context.Context, string, time.Time) (Item, error)
}

// Readiness 是 HTTP 管理面判断持久化依赖是否可用的最小契约。
type Readiness interface{ Ready(context.Context) error }

// Service 协调 Work Item 业务规则、时间与 ID，但不知道持久化和 HTTP 实现。
type Service struct {
	repository Repository
	clock      clock.Clock
	ids        idgen.Generator
	logger     logging.Logger
}

// NewService 使用消费者契约构造应用服务，不建立外部资源。
func NewService(repository Repository, appClock clock.Clock, ids idgen.Generator, logger logging.Logger) *Service {
	return &Service{repository: repository, clock: appClock, ids: ids, logger: logger.Named("workitems")}
}

// Create 校验并规范化标题，生成 Open Work Item 后持久化。
func (s *Service) Create(ctx context.Context, title string) (Item, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > MaxTitleLength {
		return Item{}, fmt.Errorf("%w: title must contain 1-%d characters", ErrInvalidTitle, MaxTitleLength)
	}
	item := Item{ID: s.ids.New(), Title: title, Status: StatusOpen, CreatedAt: s.clock.Now().UTC()}
	if err := s.repository.Create(ctx, item); err != nil {
		return Item{}, fmt.Errorf("create work item: %w", err)
	}
	s.logger.Info(ctx, "work item created", logging.String("work_item_id", item.ID))
	return item, nil
}

// Get 按 ID 返回 Work Item，并保留 Repository 的稳定错误链。
func (s *Service) Get(ctx context.Context, id string) (Item, error) {
	if strings.TrimSpace(id) == "" {
		return Item{}, ErrNotFound
	}
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return Item{}, fmt.Errorf("get work item: %w", err)
	}
	return item, nil
}

// Complete 使用当前 UTC 时间完成 Work Item；重复完成由 Repository 保持幂等。
func (s *Service) Complete(ctx context.Context, id string) (Item, error) {
	if strings.TrimSpace(id) == "" {
		return Item{}, ErrNotFound
	}
	item, err := s.repository.Complete(ctx, id, s.clock.Now().UTC())
	if err != nil {
		return Item{}, fmt.Errorf("complete work item: %w", err)
	}
	s.logger.Info(ctx, "work item completed", logging.String("work_item_id", item.ID))
	return item, nil
}
