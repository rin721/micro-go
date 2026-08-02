package crypto

// 本文件属于密码哈希组件，约束算法配置、密码长度校验、哈希生成和验证失败模式。

// 密码长度限制常量
const (
	// MinPasswordLength 最小密码长度（字节）
	MinPasswordLength = 8

	// MaxPasswordLength 最大密码长度（字节）
	// bcrypt 算法限制最大 72 字节
	MaxPasswordLength = 72

	// DefaultMinPasswordLength 默认最小密码长度
	DefaultMinPasswordLength = 8

	// DefaultMaxPasswordLength 默认最大密码长度
	DefaultMaxPasswordLength = 72
)

// Bcrypt 成本参数常量
const (
	// MinBcryptCost bcrypt 最小成本
	// 成本越小，计算速度越快，但安全性越低
	MinBcryptCost = 4

	// MaxBcryptCost bcrypt 最大成本
	// 成本越大，计算速度越慢，但安全性越高
	MaxBcryptCost = 31

	// DefaultBcryptCost 默认 bcrypt 成本
	// 推荐值，平衡安全性和性能
	DefaultBcryptCost = 10
)

// 加密算法常量
const (
	// AlgorithmBcrypt bcrypt 算法标识
	AlgorithmBcrypt = "bcrypt"

	// AlgorithmArgon2 argon2 算法标识
	AlgorithmArgon2 = "argon2"

	// DefaultAlgorithm 默认加密算法
	DefaultAlgorithm = AlgorithmBcrypt
)

// Argon2 参数常量（可选，预留）
const (
	// DefaultArgon2Time argon2 默认迭代次数
	DefaultArgon2Time = 1

	// DefaultArgon2Memory argon2 默认内存使用（KB）
	DefaultArgon2Memory = 64 * 1024 // 64MB

	// DefaultArgon2Threads argon2 默认并行度
	DefaultArgon2Threads = 4

	// DefaultArgon2KeyLen argon2 默认密钥长度
	DefaultArgon2KeyLen = 32
)
