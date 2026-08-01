// Package sqliteworkitems 使用 SQLite 实现 Work Item Repository 与 Readiness。
package sqliteworkitems

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rin721/micro-go/internal/acceptance/workitems"
	_ "modernc.org/sqlite"
)

const migrationVersion = 1

// Config 保存 SQLite 文件或 DSN；具体值只在 Adapter 内使用。
type Config struct {
	Path string `yaml:"path" json:"path" validate:"required"`
}

// Store 使用单连接 database/sql 池拥有 SQLite 资源并实现业务契约。
type Store struct {
	config Config
	mu     sync.RWMutex
	db     *sql.DB
}

// New 只保存配置，不在 Provider 构造阶段打开数据库。
func New(config Config) *Store { return &Store{config: config} }

// Prepare 打开数据库、验证连接并事务性应用当前迁移。
func (s *Store) Prepare(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return errors.New("work item database is already prepared")
	}
	database, err := sql.Open("sqlite", s.config.Path)
	if err != nil {
		return fmt.Errorf("open work item database: %w", err)
	}
	database.SetMaxOpenConns(1)
	if err := database.PingContext(ctx); err != nil {
		return errors.Join(fmt.Errorf("ping work item database: %w", err), database.Close())
	}
	if err := migrate(ctx, database); err != nil {
		return errors.Join(err, database.Close())
	}
	s.db = database
	return nil
}

func migrate(ctx context.Context, database *sql.DB) (resultErr error) {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin work item migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback work item migration: %w", rollbackErr))
			}
		}
	}()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS work_items (
            id TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            status TEXT NOT NULL CHECK (status IN ('open', 'completed')),
            created_at TEXT NOT NULL,
            completed_at TEXT NULL
        )`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
	}
	for index, statement := range statements {
		var executeErr error
		if index == len(statements)-1 {
			_, executeErr = transaction.ExecContext(ctx, statement, migrationVersion, time.Now().UTC().Format(time.RFC3339Nano))
		} else {
			_, executeErr = transaction.ExecContext(ctx, statement)
		}
		if executeErr != nil {
			return fmt.Errorf("apply work item migration %d statement %d: %w", migrationVersion, index+1, executeErr)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit work item migration: %w", err)
	}
	committed = true
	return nil
}

// Create 持久化一个已经通过应用服务校验的 Work Item。
func (s *Store) Create(ctx context.Context, item workitems.Item) error {
	database, err := s.database()
	if err != nil {
		return err
	}
	_, err = database.ExecContext(ctx,
		`INSERT INTO work_items(id, title, status, created_at, completed_at) VALUES (?, ?, ?, ?, NULL)`,
		item.ID, item.Title, item.Status, item.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert work item: %w", err)
	}
	return nil
}

// Get 按 ID 查询并把 sql.ErrNoRows 转换为 workitems.ErrNotFound。
func (s *Store) Get(ctx context.Context, id string) (workitems.Item, error) {
	database, err := s.database()
	if err != nil {
		return workitems.Item{}, err
	}
	return scanItem(database.QueryRowContext(ctx,
		`SELECT id, title, status, created_at, completed_at FROM work_items WHERE id = ?`, id,
	))
}

// Complete 在单个事务中读取状态并幂等写入完成时间。
func (s *Store) Complete(ctx context.Context, id string, completedAt time.Time) (result workitems.Item, resultErr error) {
	database, err := s.database()
	if err != nil {
		return workitems.Item{}, err
	}
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return workitems.Item{}, fmt.Errorf("begin complete work item: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback complete work item: %w", rollbackErr))
			}
		}
	}()
	item, err := scanItem(transaction.QueryRowContext(ctx,
		`SELECT id, title, status, created_at, completed_at FROM work_items WHERE id = ?`, id,
	))
	if err != nil {
		return workitems.Item{}, err
	}
	if item.Status == workitems.StatusCompleted {
		if err := transaction.Commit(); err != nil {
			return workitems.Item{}, fmt.Errorf("commit idempotent complete: %w", err)
		}
		committed = true
		return item, nil
	}
	completedAt = completedAt.UTC()
	if _, err := transaction.ExecContext(ctx,
		`UPDATE work_items SET status = ?, completed_at = ? WHERE id = ?`,
		workitems.StatusCompleted, completedAt.Format(time.RFC3339Nano), id,
	); err != nil {
		return workitems.Item{}, fmt.Errorf("update completed work item: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return workitems.Item{}, fmt.Errorf("commit complete work item: %w", err)
	}
	committed = true
	item.Status = workitems.StatusCompleted
	item.CompletedAt = &completedAt
	return item, nil
}

func scanItem(row interface{ Scan(...any) error }) (workitems.Item, error) {
	var item workitems.Item
	var status string
	var createdAt string
	var completedAt sql.NullString
	if err := row.Scan(&item.ID, &item.Title, &status, &createdAt, &completedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workitems.Item{}, workitems.ErrNotFound
		}
		return workitems.Item{}, fmt.Errorf("scan work item: %w", err)
	}
	item.Status = workitems.Status(status)
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return workitems.Item{}, fmt.Errorf("parse work item created_at: %w", err)
	}
	item.CreatedAt = parsedCreatedAt
	if completedAt.Valid {
		parsedCompletedAt, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return workitems.Item{}, fmt.Errorf("parse work item completed_at: %w", err)
		}
		item.CompletedAt = &parsedCompletedAt
	}
	return item, nil
}

// Ready 使用调用方预算验证当前数据库连接可查询。
func (s *Store) Ready(ctx context.Context) error {
	database, err := s.database()
	if err != nil {
		return err
	}
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping work item database: %w", err)
	}
	return nil
}

// Close 释放 Store 拥有的连接池；重复调用保持幂等。
func (s *Store) Close(context.Context) error {
	s.mu.Lock()
	database := s.db
	s.db = nil
	s.mu.Unlock()
	if database == nil {
		return nil
	}
	if err := database.Close(); err != nil {
		return fmt.Errorf("close work item database: %w", err)
	}
	return nil
}

func (s *Store) database() (*sql.DB, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("work item database is not prepared")
	}
	return s.db, nil
}

var _ workitems.Repository = (*Store)(nil)
var _ workitems.Readiness = (*Store)(nil)
