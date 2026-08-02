package crypto

// 本文件属于密码哈希组件，约束算法配置、密码长度校验、哈希生成和验证失败模式。

// Crypto 密码加密接口
// 定义密码加密和验证的统一接口
// 设计目标:
//   - 抽象加密算法实现细节
//   - 支持多种加密算法（bcrypt、argon2）
//   - 支持配置原子化更新
//
// 线程安全:
//
//	所有方法都是并发安全的
//
// 使用示例:
//
//	// 创建加密器
//	crypto, err := NewBcrypt()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 加密密码
//	hash, err := crypto.HashPassword("mypassword123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 验证密码
//	err = crypto.VerifyPassword(hash, "mypassword123")
//	if err != nil {
//	    log.Println("密码错误")
//	} else {
//	    log.Println("密码正确")
//	}
type Crypto interface {
	// HashPassword 加密密码
	// 将明文密码转换为安全的哈希值
	// 使用加盐哈希确保相同密码产生不同的哈希值
	// 参数:
	//   password: 明文密码
	// 返回:
	//   string: 密码哈希值（包含盐值和算法信息）
	//   error: 加密失败的错误
	// 示例:
	//   hash, err := crypto.HashPassword("mypassword123")
	//   // hash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	HashPassword(password string) (string, error)

	// VerifyPassword 验证密码
	// 比较明文密码与哈希值是否匹配
	// 参数:
	//   hashedPassword: 存储的密码哈希值
	//   password: 待验证的明文密码
	// 返回:
	//   error: nil 表示密码正确，非 nil 表示密码错误或验证失败
	// 错误类型:
	//   - ErrInvalidPassword: 密码不匹配
	//   - ErrVerificationFailed: 验证过程失败
	// 示例:
	//   err := crypto.VerifyPassword(storedHash, inputPassword)
	//   if err != nil {
	//       // 密码错误
	//   }
	VerifyPassword(hashedPassword, password string) error

	// UpdateConfig 原子化更新配置
	// 动态更新加密器配置，不影响正在进行的操作
	// 参数:
	//   opts: 可变配置选项
	// 返回:
	//   error: 配置无效或更新失败的错误
	// 使用场景:
	//   - 调整 bcrypt 成本参数
	//   - 修改密码长度限制
	//   - 切换加密算法（如支持）
	// 示例:
	//   err := crypto.UpdateConfig(
	//       WithBcryptCost(12),
	//       WithPasswordLength(10, 72),
	//   )
	UpdateConfig(opts ...Option) error
}
