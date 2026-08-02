/*
Package crypto 提供安全的密码加密和验证功能

# 设计目标

- 安全性: 使用成熟的加密算法（bcrypt、argon2）
- 易用性: 简洁的接口，合理的默认配置
- 灵活性: 支持多种算法和配置选项
- 性能: 线程安全，支持并发操作
- 可维护性: 清晰的错误处理和文档

# 核心概念

## 密码哈希

密码哈希是一种单向加密算法，将明文密码转换为不可逆的哈希值。
与普通哈希不同，密码哈希算法包含以下特性：

1. 加盐（Salting）: 为每个密码生成随机盐值，确保相同密码产生不同哈希
2. 慢速计算: 通过增加计算成本，防止暴力破解
3. 自适应: 可以调整计算成本，随着硬件发展保持安全性

## 支持的算法

### Bcrypt

Bcrypt 是目前最广泛使用的密码哈希算法之一。

优点:
  - 成熟稳定，经过长时间验证
  - 广泛支持，兼容性好
  - 自动加盐
  - 可调节成本参数

限制:
  - 最大密码长度 72 字节
  - 计算速度相对较慢（这是特性，不是缺点）

适用场景:
  - 通用密码存储
  - Web 应用认证
  - API 密钥存储

### Argon2（可选）

Argon2 是 2015 年密码哈希竞赛的冠军，是目前最先进的密码哈希算法。

优点:
  - 更强的抗 GPU 和 ASIC 攻击能力
  - 可配置内存使用，增加破解难度
  - 支持并行计算

适用场景:
  - 高安全性要求
  - 需要抵御 GPU 破解
  - 现代应用

# 使用示例

## 基本用法

创建加密器并加密密码:

	import "github.com/open-console/console-platform/pkg/crypto"

	// 创建加密器（使用默认配置）
	crypto, err := crypto.NewBcrypt()
	if err != nil {
	    log.Fatal(err)
	}

	// 加密密码
	hash, err := crypto.HashPassword("mypassword123")
	if err != nil {
	    log.Fatal(err)
	}
	fmt.Println("Password hash:", hash)

	// 验证密码
	err = crypto.VerifyPassword(hash, "mypassword123")
	if err != nil {
	    log.Println("密码错误")
	} else {
	    log.Println("密码正确")
	}

## 自定义配置

使用 Option 模式自定义配置:

	// 创建加密器，自定义成本和密码长度
	crypto, err := crypto.NewBcrypt(
	    crypto.WithBcryptCost(12),           // 增加成本，提高安全性
	    crypto.WithPasswordLength(10, 64),   // 最小 10 字符，最大 64 字符
	)
	if err != nil {
	    log.Fatal(err)
	}

## 动态更新配置

在运行时更新配置:

	// 创建加密器
	crypto, err := crypto.NewBcrypt()
	if err != nil {
	    log.Fatal(err)
	}

	// 运行时更新配置
	err = crypto.UpdateConfig(
	    crypto.WithBcryptCost(12),
	    crypto.WithPasswordLength(12, 72),
	)
	if err != nil {
	    log.Fatal(err)
	}

	// 后续加密将使用新配置
	hash, _ := crypto.HashPassword("newpassword")

## 在服务中使用

集成到 Service 层:

	type UserService struct {
	    crypto crypto.Crypto
	    repo   UserRepository
	}

	func NewUserService(crypto crypto.Crypto, repo UserRepository) *UserService {
	    return &UserService{
	        crypto: crypto,
	        repo:   repo,
	    }
	}

	func (s *UserService) Register(username, password string) error {
	    // 加密密码
	    hashedPassword, err := s.crypto.HashPassword(password)
	    if err != nil {
	        return err
	    }

	    // 保存用户
	    user := &User{
	        Username: username,
	        Password: hashedPassword,
	    }
	    return s.repo.Create(user)
	}

	func (s *UserService) Login(username, password string) error {
	    // 查询用户
	    user, err := s.repo.FindByUsername(username)
	    if err != nil {
	        return err
	    }

	    // 验证密码
	    return s.crypto.VerifyPassword(user.Password, password)
	}

# 最佳实践

## 1. 成本参数选择

Bcrypt 成本参数决定了计算时间，建议根据场景选择:

  - 开发/测试: 成本 4-6（快速）
  - 一般应用: 成本 10-12（推荐）
  - 高安全应用: 成本 12-14（更安全）

成本每增加 1，计算时间翻倍。建议在目标硬件上测试，确保登录响应时间在可接受范围内。

	// 测试不同成本的性能
	for cost := 10; cost <= 14; cost++ {
	    crypto, _ := crypto.NewBcrypt(crypto.WithBcryptCost(cost))
	    start := time.Now()
	    crypto.HashPassword("testpassword")
	    fmt.Printf("Cost %d: %v\n", cost, time.Since(start))
	}

## 2. 密码长度限制

建议设置合理的密码长度限制:

  - 最小长度: 8-12 字符（推荐 8）
  - 最大长度: 不超过 72 字符（bcrypt 限制）

过短的密码容易被破解，过长的密码可能影响用户体验。

	crypto, err := crypto.NewBcrypt(
	    crypto.WithPasswordLength(8, 72),
	)

## 3. 错误处理

正确处理加密和验证错误:

	import "errors"

	hash, err := crypto.HashPassword(password)
	if err != nil {
	    if errors.Is(err, crypto.ErrPasswordTooShort) {
	        return errors.New("密码长度至少 8 个字符")
	    }
	    if errors.Is(err, crypto.ErrPasswordTooLong) {
	        return errors.New("密码长度不能超过 72 个字符")
	    }
	    return errors.New("密码加密失败")
	}

	err = crypto.VerifyPassword(hash, inputPassword)
	if errors.Is(err, crypto.ErrInvalidPassword) {
	    return errors.New("用户名或密码错误")
	}

## 4. 安全注意事项

密码哈希存储安全:
  - 永远不要存储明文密码
  - 使用 HTTPS 传输密码
  - 使用加盐哈希（bcrypt 自动加盐）
  - 定期评估和更新成本参数

防止时序攻击:

  - 验证失败时不要透露具体原因（用户不存在 vs 密码错误）

  - 使用统一的错误消息

    // 不好的做法
    if user == nil {
    return errors.New("用户不存在")
    }
    if crypto.VerifyPassword(user.Password, password) != nil {
    return errors.New("密码错误")
    }

    // 好的做法
    if user == nil || crypto.VerifyPassword(user.Password, password) != nil {
    return errors.New("用户名或密码错误")
    }

# 线程安全

所有方法都是线程安全的，可以在并发环境下安全使用:

  - HashPassword: 并发安全
  - VerifyPassword: 并发安全
  - UpdateConfig: 使用读写锁保护，原子更新

内部使用 sync.RWMutex 保护配置的读写:
  - 读操作（加密、验证）: 使用读锁，允许并发
  - 写操作（更新配置）: 使用写锁，独占访问

# 性能考虑

## 计算成本

Bcrypt 是 CPU 密集型算法，成本每增加 1，计算时间翻倍:

	Cost  | 时间（约）  | 适用场景
	------|-----------|----------
	4     | ~5ms      | 开发/测试
	10    | ~70ms     | 一般应用
	12    | ~280ms    | 推荐值
	14    | ~1.1s     | 高安全应用

## 优化建议

1. 异步加密: 密码加密较慢，考虑在后台任务中处理
2. 缓存验证: 对于频繁验证的场景，考虑短期缓存验证结果
3. 成本平衡: 在安全性和用户体验之间找到平衡点

# 与其他包的配合

## 与 pkg/cache 配合

缓存密码验证结果（谨慎使用）:

	import "github.com/open-console/console-platform/pkg/cache"

	func verifyPasswordWithCache(cache cache.Cache, crypto crypto.Crypto, userID int64, password string) error {
	    // 注意: 缓存密码验证结果需要谨慎，可能带来安全风险
	    // 仅在特定场景下使用，如短时间内多次验证
	    cacheKey := fmt.Sprintf("pwd_verify:%d:%s", userID, hashString(password))

	    // 检查缓存
	    if cached, _ := cache.Get(ctx, cacheKey); cached != "" {
	        return nil // 缓存命中，验证成功
	    }

	    // 执行验证
	    err := crypto.VerifyPassword(storedHash, password)
	    if err == nil {
	        // 短期缓存（5分钟）
	        cache.Set(ctx, cacheKey, "1", 5*time.Minute)
	    }
	    return err
	}

## 与 pkg/logger 配合

记录关键操作:

	import "github.com/open-console/console-platform/pkg/logger"

	func hashPasswordWithLogging(log logger.Logger, crypto crypto.Crypto, password string) (string, error) {
	    hash, err := crypto.HashPassword(password)
	    if err != nil {
	        log.Error("password hashing failed", "error", err)
	        return "", err
	    }
	    log.Debug("password hashed successfully")
	    return hash, nil
	}

# 错误参考

预定义错误:

  - ErrInvalidPassword: 密码不匹配
  - ErrPasswordTooShort: 密码长度不足
  - ErrPasswordTooLong: 密码长度超出限制
  - ErrHashingFailed: 加密失败
  - ErrVerificationFailed: 验证失败
  - ErrInvalidConfig: 配置无效
  - ErrInvalidAlgorithm: 算法无效

使用示例:

	import "errors"

	err := crypto.VerifyPassword(hash, password)
	if errors.Is(err, crypto.ErrInvalidPassword) {
	    // 密码不匹配
	}

# 参考链接

  - Bcrypt 原理: https://en.wikipedia.org/wiki/Bcrypt
  - OWASP 密码存储: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
  - golang.org/x/crypto: https://pkg.go.dev/golang.org/x/crypto
*/
package crypto

// 本文件承载包级 Godoc 入口，集中说明该包在脚手架架构中的定位、使用边界和非目标能力。
