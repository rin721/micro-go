package crypto

// 本文件属于密码哈希组件，约束算法配置、密码长度校验、哈希生成和验证失败模式。

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCrypto bcrypt 密码加密器实现
// 实现 Crypto 接口，使用 bcrypt 算法
type bcryptCrypto struct {
	mu     sync.RWMutex // 保护配置的读写锁
	config *Config      // 当前配置
}

// NewBcrypt 创建 bcrypt 密码加密器
// 参数:
//
//	opts: 可选配置选项
//
// 返回:
//
//	Crypto: 加密器实例
//	error: 配置无效时的错误
//
// 使用示例:
//
//	// 使用默认配置
//	crypto, err := NewBcrypt()
//
//	// 自定义配置
//	crypto, err := NewBcrypt(
//	    WithBcryptCost(12),
//	    WithPasswordLength(10, 72),
//	)
func NewBcrypt(opts ...Option) (Crypto, error) {
	// 创建默认配置
	config := DefaultConfig()
	config.Algorithm = AlgorithmBcrypt

	// 应用用户配置
	for _, opt := range opts {
		opt(config)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf(ErrMsgInvalidConfig, err)
	}

	return &bcryptCrypto{
		config: config,
	}, nil
}

// HashPassword 实现 Crypto 接口
// 使用 bcrypt 算法加密密码
func (b *bcryptCrypto) HashPassword(password string) (string, error) {
	// 读取当前配置
	b.mu.RLock()
	config := b.config
	b.mu.RUnlock()

	// 验证密码长度
	if err := b.validatePassword(password); err != nil {
		return "", err
	}

	// 使用 bcrypt 加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), config.BcryptCost)
	if err != nil {
		return "", fmt.Errorf(ErrMsgHashingFailed, err)
	}

	return string(hashedPassword), nil
}

// VerifyPassword 实现 Crypto 接口
// 使用 bcrypt 算法验证密码
func (b *bcryptCrypto) VerifyPassword(hashedPassword, password string) error {
	// 验证密码长度（可选，bcrypt 会自动验证）
	if err := b.validatePassword(password); err != nil {
		return err
	}

	// 使用 bcrypt 验证
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		// bcrypt.ErrMismatchedHashAndPassword 表示密码不匹配
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return ErrInvalidPassword
		}
		return fmt.Errorf(ErrMsgVerificationFailed, err)
	}

	return nil
}

// UpdateConfig 实现 Crypto 接口
// 原子化更新配置
func (b *bcryptCrypto) UpdateConfig(opts ...Option) error {
	// 克隆当前配置
	b.mu.RLock()
	newConfig := b.config.Clone()
	b.mu.RUnlock()

	// 应用新配置选项
	for _, opt := range opts {
		opt(newConfig)
	}

	// 验证新配置
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf(ErrMsgInvalidConfig, err)
	}

	// 原子替换配置
	b.mu.Lock()
	b.config = newConfig
	b.mu.Unlock()

	return nil
}

// validatePassword 验证密码长度
// 根据配置的长度限制检查密码是否合法
func (b *bcryptCrypto) validatePassword(password string) error {
	b.mu.RLock()
	minLen := b.config.MinPasswordLength
	maxLen := b.config.MaxPasswordLength
	b.mu.RUnlock()

	length := len(password)

	if length < minLen {
		return fmt.Errorf(ErrMsgPasswordTooShort, minLen)
	}

	if length > maxLen {
		return fmt.Errorf(ErrMsgPasswordTooLong, maxLen)
	}

	return nil
}
