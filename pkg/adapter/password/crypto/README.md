# pkg/crypto - 密码加密工具库

安全、易用的密码加密和验证库，基于成熟的 bcrypt 算法。

## API 分类

- 定位：[CONFIRMED] 公共基础设施 API。
- 稳定边界：`Crypto` 接口、bcrypt 实现、`Config`、Option 配置函数。
- 当前风险：[RISK] 当前稳定实现仅覆盖 bcrypt，其他算法配置属于扩展预留。
- 非目标：[CONFIRMED] 本包不负责认证流程、会话、JWT 或 RBAC。

## 特性

- ✅ **安全可靠** - 使用 bcrypt 算法，自动加盐
- ✅ **接口抽象** - 统一接口，支持多种算法
- ✅ **配置灵活** - Option 模式，支持动态更新配置
- ✅ **线程安全** - 所有操作并发安全
- ✅ **详细注释** - 完整的中文注释和文档
- ✅ **测试完善** - 单元测试覆盖核心功能

## 安装

```bash
go get golang.org/x/crypto
```

## 快速开始

### 1. 创建加密器

```go
package main

import (
    "log"
    "github.com/open-console/console-platform/pkg/crypto"
)

func main() {
    // 使用默认配置
    crypto, err := crypto.NewBcrypt()
    if err != nil {
        log.Fatal(err)
    }
}
```

### 2. 加密密码

```go
// 用户注册时加密密码
hash, err := crypto.HashPassword("mypassword123")
if err != nil {
    log.Fatal(err)
}

fmt.Println("Password hash:", hash)
// 输出: Password hash: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
```

### 3. 验证密码

```go
// 用户登录时验证密码
err = crypto.VerifyPassword(hash, "mypassword123")
if err != nil {
    log.Println("密码错误")
} else {
    log.Println("密码正确")
}
```

### 4. 自定义配置

```go
// 创建加密器，自定义成本和密码长度
crypto, err := crypto.NewBcrypt(
    crypto.WithBcryptCost(12),           // 增加成本，提高安全性
    crypto.WithPasswordLength(10, 64),   // 最小 10 字符，最大 64 字符
)
```

### 5. 动态更新配置

```go
// 运行时更新配置
err = crypto.UpdateConfig(
    crypto.WithBcryptCost(12),
    crypto.WithPasswordLength(12, 72),
)
```

## API 参考

### 接口定义

```go
type Crypto interface {
    HashPassword(password string) (string, error)
    VerifyPassword(hashedPassword, password string) error
    UpdateConfig(opts ...Option) error
}
```

### 配置选项

| Option                             | 说明                    | 示例                        |
| ---------------------------------- | ----------------------- | --------------------------- |
| `WithBcryptCost(cost int)`         | 设置 bcrypt 成本 (4-31) | `WithBcryptCost(12)`        |
| `WithPasswordLength(min, max int)` | 设置密码长度限制        | `WithPasswordLength(8, 72)` |
| `WithAlgorithm(algo string)`       | 设置加密算法            | `WithAlgorithm("bcrypt")`   |

### 配置结构

```go
type Config struct {
    Algorithm         string  // 加密算法（默认: "bcrypt"）
    BcryptCost        int     // bcrypt 成本（默认: 10）
    MinPasswordLength int     // 最小密码长度（默认: 8）
    MaxPasswordLength int     // 最大密码长度（默认: 72）
}
```

## 使用场景

### 场景 1: 用户注册

```go
type UserService struct {
    crypto crypto.Crypto
    repo   UserRepository
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
```

### 场景 2: 用户登录

```go
func (s *UserService) Login(username, password string) error {
    // 查询用户
    user, err := s.repo.FindByUsername(username)
    if err != nil {
        return err
    }

    // 验证密码
    err = s.crypto.VerifyPassword(user.Password, password)
    if err != nil {
        return errors.New("用户名或密码错误")
    }

    return nil
}
```

### 场景 3: 密码重置

```go
func (s *UserService) ResetPassword(userID int64, newPassword string) error {
    // 加密新密码
    hashedPassword, err := s.crypto.HashPassword(newPassword)
    if err != nil {
        return err
    }

    // 更新密码
    return s.repo.UpdatePassword(userID, hashedPassword)
}
```

## 配置说明

### Bcrypt 成本参数

成本参数决定了计算时间，建议根据场景选择：

| 成本 | 计算时间（约） | 适用场景         |
| ---- | -------------- | ---------------- |
| 4    | ~5ms           | 开发/测试        |
| 10   | ~70ms          | 一般应用（推荐） |
| 12   | ~280ms         | 高安全应用       |
| 14   | ~1.1s          | 极高安全应用     |

**建议**：

- 开发环境：成本 4-6（快速）
- 生产环境：成本 10-12（推荐）
- 高安全应用：成本 12-14（更安全）

成本每增加 1，计算时间翻倍。

### 密码长度限制

建议设置合理的密码长度限制：

| 参数     | 推荐值         | 说明         |
| -------- | -------------- | ------------ |
| 最小长度 | 8-12 字符      | 确保密码强度 |
| 最大长度 | 不超过 72 字符 | bcrypt 限制  |

**注意**：bcrypt 算法限制最大密码长度为 72 字节。

## 最佳实践

### 1. 成本参数选择

```go
// 在目标硬件上测试性能
for cost := 10; cost <= 14; cost++ {
    crypto, _ := crypto.NewBcrypt(crypto.WithBcryptCost(cost))
    start := time.Now()
    crypto.HashPassword("testpassword")
    fmt.Printf("Cost %d: %v\n", cost, time.Since(start))
}
```

### 2. 错误处理

```go
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
```

### 3. 防止时序攻击

```go
// ✅ 好的做法 - 统一错误消息
if user == nil || crypto.VerifyPassword(user.Password, password) != nil {
    return errors.New("用户名或密码错误")
}

// ❌ 不好的做法 - 暴露具体原因
if user == nil {
    return errors.New("用户不存在")
}
if crypto.VerifyPassword(user.Password, password) != nil {
    return errors.New("密码错误")
}
```

### 4. 安全注意事项

**密码存储安全**：

- ✅ 永远不要存储明文密码
- ✅ 使用 HTTPS 传输密码
- ✅ 使用加盐哈希（bcrypt 自动加盐）
- ✅ 定期评估和更新成本参数

**相同密码产生不同哈希**：

```go
hash1, _ := crypto.HashPassword("samepassword")
hash2, _ := crypto.HashPassword("samepassword")
// hash1 != hash2 （因为盐值不同）
```

## 错误参考

### 预定义错误

| 错误                    | 说明       | 场景             |
| ----------------------- | ---------- | ---------------- |
| `ErrInvalidPassword`    | 密码不匹配 | 验证密码失败     |
| `ErrPasswordTooShort`   | 密码过短   | 密码长度不足     |
| `ErrPasswordTooLong`    | 密码过长   | 密码长度超出限制 |
| `ErrHashingFailed`      | 加密失败   | 加密过程出错     |
| `ErrVerificationFailed` | 验证失败   | 验证过程出错     |
| `ErrInvalidConfig`      | 配置无效   | 配置参数不合法   |

### 错误处理示例

```go
import "errors"

err := crypto.VerifyPassword(hash, password)
if errors.Is(err, crypto.ErrInvalidPassword) {
    // 密码不匹配
}
```

## 性能考虑

### 计算成本

Bcrypt 是 CPU 密集型算法，成本每增加 1，计算时间翻倍：

```
Cost  | 时间（约）  | 适用场景
------|-----------|----------
4     | ~5ms      | 开发/测试
10    | ~70ms     | 一般应用
12    | ~280ms    | 推荐值
14    | ~1.1s     | 高安全应用
```

### 性能基准测试

```bash
go test -bench=. -benchmem ./pkg/crypto
```

## 常量定义

### 默认配置

```go
DefaultAlgorithm         = "bcrypt"
DefaultBcryptCost        = 10
DefaultMinPasswordLength = 8
DefaultMaxPasswordLength = 72
```

### 成本限制

```go
MinBcryptCost = 4
MaxBcryptCost = 31
```

## 线程安全

所有方法都是线程安全的，可以在并发环境下安全使用：

- `HashPassword`: 并发安全
- `VerifyPassword`: 并发安全
- `UpdateConfig`: 使用读写锁保护，原子更新

内部使用 `sync.RWMutex` 保护配置：

- 读操作（加密、验证）：使用读锁，允许并发
- 写操作（更新配置）：使用写锁，独占访问

## 项目结构

```
pkg/crypto/
├── doc.go          # 包文档
├── README.md       # 本文档
├── errors.go       # 错误定义
├── constants.go    # 常量定义
├── config.go       # 配置结构
├── crypto.go       # 接口定义
├── bcrypt_impl.go  # bcrypt 实现
├── crypto_test.go  # 单元测试
└── examples/       # 示例代码
    ├── README.md
    └── basic/
        └── main.go
```

## 依赖项

- `golang.org/x/crypto/bcrypt` - bcrypt 算法实现

## 参考链接

- [Bcrypt 原理](https://en.wikipedia.org/wiki/Bcrypt)
- [OWASP 密码存储](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)
