package crypto

// 本文件属于密码哈希组件，约束算法配置、密码长度校验、哈希生成和验证失败模式。

import (
	"fmt"
)

// Config 密码加密配置
type Config struct {
	// Algorithm 加密算法
	// 可选值: "bcrypt", "argon2"
	// 默认: "bcrypt"
	Algorithm string

	// BcryptCost bcrypt 成本参数
	// 范围: 4-31
	// 默认: 10
	// 成本越高，计算时间越长，安全性越高
	BcryptCost int

	// Argon2Time argon2 迭代次数（可选，预留）
	Argon2Time uint32

	// Argon2Memory argon2 内存使用（KB）（可选，预留）
	Argon2Memory uint32

	// Argon2Threads argon2 并行度（可选，预留）
	Argon2Threads uint8

	// Argon2KeyLen argon2 密钥长度（可选，预留）
	Argon2KeyLen uint32

	// MinPasswordLength 最小密码长度
	// 默认: 8
	MinPasswordLength int

	// MaxPasswordLength 最大密码长度
	// 默认: 72（bcrypt 限制）
	MaxPasswordLength int
}

// DefaultConfig 返回默认配置
// 使用 bcrypt 算法，成本为 10，密码长度 8-72 字节
func DefaultConfig() *Config {
	return &Config{
		Algorithm:         DefaultAlgorithm,
		BcryptCost:        DefaultBcryptCost,
		Argon2Time:        DefaultArgon2Time,
		Argon2Memory:      DefaultArgon2Memory,
		Argon2Threads:     DefaultArgon2Threads,
		Argon2KeyLen:      DefaultArgon2KeyLen,
		MinPasswordLength: DefaultMinPasswordLength,
		MaxPasswordLength: DefaultMaxPasswordLength,
	}
}

// Validate 验证配置有效性
// 检查算法、成本参数、密码长度限制是否合法
// 返回:
//
//	error: 配置无效时的错误
func (c *Config) Validate() error {
	// 验证算法
	switch c.Algorithm {
	case AlgorithmBcrypt:
		// bcrypt 算法，验证成本参数
		if c.BcryptCost < MinBcryptCost || c.BcryptCost > MaxBcryptCost {
			return fmt.Errorf("%w: bcrypt cost must be between %d and %d, got %d",
				ErrInvalidConfig, MinBcryptCost, MaxBcryptCost, c.BcryptCost)
		}
	case AlgorithmArgon2:
		// argon2 算法（预留）
		if c.Argon2Time == 0 {
			return fmt.Errorf("%w: argon2 time must be greater than 0", ErrInvalidConfig)
		}
		if c.Argon2Memory == 0 {
			return fmt.Errorf("%w: argon2 memory must be greater than 0", ErrInvalidConfig)
		}
		if c.Argon2Threads == 0 {
			return fmt.Errorf("%w: argon2 threads must be greater than 0", ErrInvalidConfig)
		}
		if c.Argon2KeyLen == 0 {
			return fmt.Errorf("%w: argon2 key length must be greater than 0", ErrInvalidConfig)
		}
	default:
		return fmt.Errorf(ErrMsgInvalidAlgorithm, c.Algorithm, []string{AlgorithmBcrypt, AlgorithmArgon2})
	}

	// 验证密码长度限制
	if c.MinPasswordLength < 1 {
		return fmt.Errorf("%w: min password length must be at least 1", ErrInvalidConfig)
	}
	if c.MaxPasswordLength < c.MinPasswordLength {
		return fmt.Errorf("%w: max password length must be greater than or equal to min password length", ErrInvalidConfig)
	}
	if c.MaxPasswordLength > MaxPasswordLength {
		return fmt.Errorf("%w: max password length cannot exceed %d (bcrypt limit)", ErrInvalidConfig, MaxPasswordLength)
	}

	return nil
}

// Clone 克隆配置
// 用于原子化更新配置时创建新配置副本
func (c *Config) Clone() *Config {
	return &Config{
		Algorithm:         c.Algorithm,
		BcryptCost:        c.BcryptCost,
		Argon2Time:        c.Argon2Time,
		Argon2Memory:      c.Argon2Memory,
		Argon2Threads:     c.Argon2Threads,
		Argon2KeyLen:      c.Argon2KeyLen,
		MinPasswordLength: c.MinPasswordLength,
		MaxPasswordLength: c.MaxPasswordLength,
	}
}

// Option 配置选项函数类型
// 用于原子化更新配置参数
type Option func(*Config)

// WithAlgorithm 设置加密算法
// 参数:
//
//	algo: 算法名称，可选值 "bcrypt" 或 "argon2"
func WithAlgorithm(algo string) Option {
	return func(c *Config) {
		c.Algorithm = algo
	}
}

// WithBcryptCost 设置 bcrypt 成本
// 参数:
//
//	cost: 成本参数，范围 4-31
func WithBcryptCost(cost int) Option {
	return func(c *Config) {
		c.BcryptCost = cost
	}
}

// WithPasswordLength 设置密码长度限制
// 参数:
//
//	min: 最小长度
//	max: 最大长度
func WithPasswordLength(min, max int) Option {
	return func(c *Config) {
		c.MinPasswordLength = min
		c.MaxPasswordLength = max
	}
}

// WithArgon2Params 设置 argon2 参数（预留）
// 参数:
//
//	time: 迭代次数
//	memory: 内存使用（KB）
//	threads: 并行度
//	keyLen: 密钥长度
func WithArgon2Params(time uint32, memory uint32, threads uint8, keyLen uint32) Option {
	return func(c *Config) {
		c.Argon2Time = time
		c.Argon2Memory = memory
		c.Argon2Threads = threads
		c.Argon2KeyLen = keyLen
	}
}
