# 示例代码

## 运行示例

### 基础示例

演示 pkg/crypto 的基本功能：

```bash
cd pkg/crypto/examples/basic
go run main.go
```

**示例输出**：

```
=== pkg/crypto 基础示例 ===

1. 创建加密器
✓ 加密器创建成功（使用默认配置）

2. 加密密码
明文密码: mypassword123
密码哈希: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
✓ 密码加密成功

3. 验证密码
验证密码: mypassword123
✓ 密码验证成功

4. 验证错误密码
验证密码: wrongpassword
✓ 密码验证失败（符合预期）: invalid password

5. 创建自定义配置的加密器
✓ 自定义加密器创建成功（成本=12，密码长度=10-64）

6. 测试密码长度验证
尝试加密过短的密码: short
✓ 密码过短，加密失败（符合预期）: password too short: minimum length is 10

7. 动态更新配置
✓ 配置更新成功（成本=11，密码长度=12-72）

8. 验证更新后的配置
新密码哈希: $2a$11$...
✓ 使用新配置加密成功

9. 相同密码产生不同哈希（加盐效果）
密码 1 哈希: $2a$11$...
密码 2 哈希: $2a$11$...
✓ 相同密码产生不同哈希（加盐生效）

10. 验证两个哈希都有效
✓ 两个哈希都能验证原密码

=== 示例运行成功 ===
```

## 示例说明

### basic 示例

演示内容：

- 创建加密器（默认配置）
- 密码加密和验证
- 自定义配置（成本、密码长度）
- 密码长度验证
- 动态更新配置
- 加盐效果演示（相同密码产生不同哈希）

## 集成示例

### 在 Service 层使用

```go
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
```

## 更多示例

查看主 README 文档了解更多使用场景：

- [pkg/crypto/README.md](../README.md)
