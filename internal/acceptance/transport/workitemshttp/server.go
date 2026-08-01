// Package workitemshttp 提供 Work Item 验收系统的标准库 HTTP 边界。
package workitemshttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rin721/micro-go/internal/acceptance/workitems"
	"github.com/rin721/micro-go/types/capability/logging"
)

// Config 定义 HTTP 监听、超时和请求体边界，所有值都必须显式校验。
type Config struct {
	ListenAddress string        `yaml:"listen_address" json:"listen_address" validate:"required"`
	ReadTimeout   time.Duration `yaml:"read_timeout" json:"read_timeout" validate:"gt=0"`
	WriteTimeout  time.Duration `yaml:"write_timeout" json:"write_timeout" validate:"gt=0"`
	IdleTimeout   time.Duration `yaml:"idle_timeout" json:"idle_timeout" validate:"gt=0"`
	HealthTimeout time.Duration `yaml:"health_timeout" json:"health_timeout" validate:"gt=0"`
	MaxBodyBytes  int64         `yaml:"max_body_bytes" json:"max_body_bytes" validate:"gt=0"`
}

// Server 拥有 Listener 和 http.Server，并把传输请求转给 Work Item 应用服务。
type Server struct {
	config    Config
	service   *workitems.Service
	readiness workitems.Readiness
	logger    logging.Logger
	http      *http.Server

	mu       sync.RWMutex
	listener net.Listener
	started  bool
}

// New 构建路由和内存 HTTP Server，不监听端口。
func New(config Config, service *workitems.Service, readiness workitems.Readiness, logger logging.Logger) *Server {
	server := &Server{config: config, service: service, readiness: readiness, logger: logger.Named("workitems.http")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", server.handleLiveness)
	mux.HandleFunc("GET /readyz", server.handleReadiness)
	mux.HandleFunc("GET /status", server.handleStatus)
	mux.HandleFunc("POST /v1/work-items", server.handleCreate)
	mux.HandleFunc("GET /v1/work-items/{id}", server.handleGet)
	mux.HandleFunc("POST /v1/work-items/{id}/complete", server.handleComplete)
	server.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: config.ReadTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
	}
	return server
}

// Prepare 建立 Listener，使端口冲突在 Runner 启动前失败。
func (s *Server) Prepare(context.Context) error {
	listener, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen for work item HTTP server: %w", err)
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()
	return nil
}

// Start 确认 Listener 已准备，并登记 Runtime 后续 Stop 的所有权。
func (s *Server) Start(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return errors.New("work item HTTP listener is not prepared")
	}
	s.started = true
	return nil
}

// Run 阻塞执行 Serve；只有根 Context 取消后的 ErrServerClosed 属于协作退出。
func (s *Server) Run(ctx context.Context) error {
	s.mu.RLock()
	listener := s.listener
	started := s.started
	s.mu.RUnlock()
	if listener == nil || !started {
		return errors.New("work item HTTP server is not started")
	}
	err := s.http.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("serve work item HTTP: %w", err)
	}
	return errors.New("work item HTTP server exited unexpectedly")
}

// Stop 使用共享 shutdown budget 停止接收请求并等待在途 Handler。
func (s *Server) Stop(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown work item HTTP server: %w", err)
	}
	return nil
}

// Close 释放 Listener；重复关闭和 Shutdown 已关闭的 Listener 不算失败。
func (s *Server) Close(context.Context) error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.started = false
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("close work item HTTP listener: %w", err)
	}
	return nil
}

// Address 返回 Prepare 后的实际监听地址，供黑盒验收使用。
func (s *Server) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) handleLiveness(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "live"}, s.logger)
}

func (s *Server) handleReadiness(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), s.config.HealthTimeout)
	defer cancel()
	if err := s.readiness.Ready(ctx); err != nil {
		s.logger.Error(request.Context(), "readiness check failed", logging.Error(err))
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"}, s.logger)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"}, s.logger)
}

func (s *Server) handleStatus(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), s.config.HealthTimeout)
	defer cancel()
	ready := s.readiness.Ready(ctx) == nil
	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()
	writeJSON(writer, http.StatusOK, map[string]any{"state": "running", "started": started, "ready": ready}, s.logger)
}

type createRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleCreate(writer http.ResponseWriter, request *http.Request) {
	if mediaType := request.Header.Get("Content-Type"); mediaType != "" && !strings.HasPrefix(strings.ToLower(mediaType), "application/json") {
		writeAPIError(writer, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json", s.logger)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, s.config.MaxBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input createRequest
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_request", "request body must be one valid JSON object", s.logger)
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_request", "request body must contain exactly one JSON object", s.logger)
		return
	}
	item, err := s.service.Create(request.Context(), input.Title)
	if err != nil {
		if errors.Is(err, workitems.ErrInvalidTitle) {
			writeAPIError(writer, http.StatusBadRequest, "invalid_title", "title must contain 1-200 characters", s.logger)
			return
		}
		s.internalError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, item, s.logger)
}

func (s *Server) handleGet(writer http.ResponseWriter, request *http.Request) {
	item, err := s.service.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		if errors.Is(err, workitems.ErrNotFound) {
			writeAPIError(writer, http.StatusNotFound, "not_found", "work item was not found", s.logger)
			return
		}
		s.internalError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, item, s.logger)
}

func (s *Server) handleComplete(writer http.ResponseWriter, request *http.Request) {
	item, err := s.service.Complete(request.Context(), request.PathValue("id"))
	if err != nil {
		if errors.Is(err, workitems.ErrNotFound) {
			writeAPIError(writer, http.StatusNotFound, "not_found", "work item was not found", s.logger)
			return
		}
		s.internalError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, item, s.logger)
}

func (s *Server) internalError(writer http.ResponseWriter, request *http.Request, err error) {
	s.logger.Error(request.Context(), "work item request failed", logging.Error(err))
	writeAPIError(writer, http.StatusInternalServerError, "internal_error", "request could not be completed", s.logger)
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("additional JSON value")
	}
	return err
}

func writeAPIError(writer http.ResponseWriter, status int, code, message string, logger logging.Logger) {
	writeJSON(writer, status, map[string]any{"error": map[string]string{"code": code, "message": message}}, logger)
}

func writeJSON(writer http.ResponseWriter, status int, value any, logger logging.Logger) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		logger.Error(context.Background(), "write HTTP JSON response", logging.Error(err))
	}
}
