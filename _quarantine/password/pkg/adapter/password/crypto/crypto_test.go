package crypto

// 本测试文件固定密码哈希组件的配置、并发和校验契约，防止注释补全和后续重构改变外部可观察行为。

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// TestNewBcrypt 测试创建 bcrypt 加密器
func TestNewBcrypt(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "默认配置",
			opts:    nil,
			wantErr: false,
		},
		{
			name: "自定义成本",
			opts: []Option{
				WithBcryptCost(12),
			},
			wantErr: false,
		},
		{
			name: "自定义密码长度",
			opts: []Option{
				WithPasswordLength(10, 50),
			},
			wantErr: false,
		},
		{
			name: "无效成本（过小）",
			opts: []Option{
				WithBcryptCost(3),
			},
			wantErr: true,
		},
		{
			name: "无效成本（过大）",
			opts: []Option{
				WithBcryptCost(32),
			},
			wantErr: true,
		},
		{
			name: "无效密码长度",
			opts: []Option{
				WithPasswordLength(100, 50), // 最小长度大于最大长度。
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crypto, err := NewBcrypt(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBcrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && crypto == nil {
				t.Error("NewBcrypt() returned nil crypto")
			}
		})
	}
}

// TestHashPassword 测试密码加密
func TestHashPassword(t *testing.T) {
	crypto, err := NewBcrypt()
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "正常密码",
			password: "mypassword123",
			wantErr:  false,
		},
		{
			name:     "最小长度密码",
			password: "12345678", // 8 字符
			wantErr:  false,
		},
		{
			name:     "长密码",
			password: strings.Repeat("a", 70),
			wantErr:  false,
		},
		{
			name:     "密码过短",
			password: "1234567", // 7 字符
			wantErr:  true,
		},
		{
			name:     "密码过长",
			password: strings.Repeat("a", 73), // 73 字符
			wantErr:  true,
		},
		{
			name:     "空密码",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := crypto.HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if hash == "" {
					t.Error("HashPassword() returned empty hash")
				}
				// bcrypt 哈希应该以 $2a$ 或 $2b$ 开头
				if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
					t.Errorf("HashPassword() returned invalid hash format: %s", hash)
				}
			}
		})
	}
}

// TestVerifyPassword 测试密码验证
func TestVerifyPassword(t *testing.T) {
	crypto, err := NewBcrypt()
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	// 加密测试密码
	password := "mypassword123"
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	tests := []struct {
		name           string
		hashedPassword string
		password       string
		wantErr        bool
		checkErrType   error
	}{
		{
			name:           "正确密码",
			hashedPassword: hash,
			password:       password,
			wantErr:        false,
		},
		{
			name:           "错误密码",
			hashedPassword: hash,
			password:       "wrongpassword",
			wantErr:        true,
			checkErrType:   ErrInvalidPassword,
		},
		{
			name:           "空密码",
			hashedPassword: hash,
			password:       "",
			wantErr:        true,
		},
		{
			name:           "无效哈希",
			hashedPassword: "invalid-hash",
			password:       password,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := crypto.VerifyPassword(tt.hashedPassword, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.checkErrType != nil && !errors.Is(err, tt.checkErrType) {
				t.Errorf("VerifyPassword() error type = %v, want %v", err, tt.checkErrType)
			}
		})
	}
}

// TestUpdateConfig 测试配置更新
func TestUpdateConfig(t *testing.T) {
	crypto, err := NewBcrypt(WithBcryptCost(10))
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	// 测试更新成本
	err = crypto.UpdateConfig(WithBcryptCost(12))
	if err != nil {
		t.Errorf("UpdateConfig() failed: %v", err)
	}

	// 测试无效配置
	err = crypto.UpdateConfig(WithBcryptCost(100))
	if err == nil {
		t.Error("UpdateConfig() should fail with invalid cost")
	}

	// 验证加密仍然正常工作
	hash, err := crypto.HashPassword("testpassword")
	if err != nil {
		t.Errorf("HashPassword() after UpdateConfig() failed: %v", err)
	}

	err = crypto.VerifyPassword(hash, "testpassword")
	if err != nil {
		t.Errorf("VerifyPassword() after UpdateConfig() failed: %v", err)
	}
}

// TestPasswordLengthValidation 测试密码长度验证
func TestPasswordLengthValidation(t *testing.T) {
	crypto, err := NewBcrypt(WithPasswordLength(10, 20))
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "长度合法",
			password: "1234567890", // 10 字符
			wantErr:  false,
		},
		{
			name:     "长度合法（最大）",
			password: strings.Repeat("a", 20), // 20 字符
			wantErr:  false,
		},
		{
			name:     "长度不足",
			password: "123456789", // 9 字符
			wantErr:  true,
		},
		{
			name:     "长度超出",
			password: strings.Repeat("a", 21), // 21 字符
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crypto.HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConcurrency 测试并发安全
func TestConcurrency(t *testing.T) {
	crypto, err := NewBcrypt()
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	// 并发加密
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			password := "password"
			hash, err := crypto.HashPassword(password)
			if err != nil {
				errors <- err
				return
			}
			err = crypto.VerifyPassword(hash, password)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// 并发更新配置
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = crypto.UpdateConfig(WithBcryptCost(11))
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

// TestHashUniqueness 测试相同密码产生不同哈希
func TestHashUniqueness(t *testing.T) {
	crypto, err := NewBcrypt()
	if err != nil {
		t.Fatalf("NewBcrypt() failed: %v", err)
	}

	password := "samepassword"
	hash1, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	hash2, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	// 相同密码应该产生不同的哈希（因为盐值不同）
	if hash1 == hash2 {
		t.Error("Same password produced identical hashes, salt may not be working")
	}

	// 但两个哈希都应该能验证原密码
	if err := crypto.VerifyPassword(hash1, password); err != nil {
		t.Errorf("VerifyPassword() failed for hash1: %v", err)
	}
	if err := crypto.VerifyPassword(hash2, password); err != nil {
		t.Errorf("VerifyPassword() failed for hash2: %v", err)
	}
}

// BenchmarkHashPassword 性能基准测试
func BenchmarkHashPassword(b *testing.B) {
	crypto, _ := NewBcrypt()
	password := "benchmarkpassword"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.HashPassword(password)
	}
}

// BenchmarkVerifyPassword 性能基准测试
func BenchmarkVerifyPassword(b *testing.B) {
	crypto, _ := NewBcrypt()
	password := "benchmarkpassword"
	hash, _ := crypto.HashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = crypto.VerifyPassword(hash, password)
	}
}
