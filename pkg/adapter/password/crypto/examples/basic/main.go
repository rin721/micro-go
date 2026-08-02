package main

// 本文件是密码哈希包的最小可运行示例，展示配置、哈希、校验和错误处理的推荐调用顺序。

import (
	"fmt"
	"log"

	"github.com/rin721/micro-go/pkg/adapter/password/crypto"
)

// main 封装当前文件的内部辅助流程，避免公开入口直接承载重复控制流。
func main() {
	fmt.Print("=== pkg/crypto 基础示例 ===\n")

	// 1. 创建加密器（使用默认配置）
	fmt.Println("1. 创建加密器")
	cryptoInstance, err := crypto.NewBcrypt()
	if err != nil {
		log.Fatalf("创建加密器失败: %v", err)
	}
	fmt.Print("✓ 加密器创建成功（使用默认配置）\n")

	// 2. 加密密码
	fmt.Println("2. 加密密码")
	password := "mypassword123"
	fmt.Printf("明文密码: %s\n", password)

	hash, err := cryptoInstance.HashPassword(password)
	if err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}
	fmt.Printf("密码哈希: %s\n", hash)
	fmt.Print("✓ 密码加密成功\n")

	// 3. 验证正确的密码
	fmt.Println("3. 验证密码")
	fmt.Printf("验证密码: %s\n", password)

	err = cryptoInstance.VerifyPassword(hash, password)
	if err != nil {
		log.Fatalf("密码验证失败: %v", err)
	}
	fmt.Print("✓ 密码验证成功\n")

	// 4. 验证错误的密码
	fmt.Println("4. 验证错误密码")
	wrongPassword := "wrongpassword"
	fmt.Printf("验证密码: %s\n", wrongPassword)

	err = cryptoInstance.VerifyPassword(hash, wrongPassword)
	if err != nil {
		fmt.Printf("✓ 密码验证失败（符合预期）: %v\n\n", err)
	} else {
		log.Fatal("错误：本应验证失败")
	}

	// 5. 自定义配置
	fmt.Println("5. 创建自定义配置的加密器")
	customCrypto, err := crypto.NewBcrypt(
		crypto.WithBcryptCost(12),         // 增加成本
		crypto.WithPasswordLength(10, 64), // 密码长度 10-64
	)
	if err != nil {
		log.Fatalf("创建加密器失败: %v", err)
	}
	fmt.Print("✓ 自定义加密器创建成功（成本=12，密码长度=10-64）\n")

	// 6. 测试密码长度验证
	fmt.Println("6. 测试密码长度验证")
	shortPassword := "short" // 少于 10 个字符
	fmt.Printf("尝试加密过短的密码: %s\n", shortPassword)

	_, err = customCrypto.HashPassword(shortPassword)
	if err != nil {
		fmt.Printf("✓ 密码过短，加密失败（符合预期）: %v\n\n", err)
	} else {
		log.Fatal("错误：本应加密失败")
	}

	// 7. 动态更新配置
	fmt.Println("7. 动态更新配置")
	err = cryptoInstance.UpdateConfig(
		crypto.WithBcryptCost(11),
		crypto.WithPasswordLength(12, 72),
	)
	if err != nil {
		log.Fatalf("更新配置失败: %v", err)
	}
	fmt.Print("✓ 配置更新成功（成本=11，密码长度=12-72）\n")

	// 8. 验证更新后的配置生效
	fmt.Println("8. 验证更新后的配置")
	newPassword := "newpassword123"
	newHash, err := cryptoInstance.HashPassword(newPassword)
	if err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}
	fmt.Printf("新密码哈希: %s\n", newHash)
	fmt.Print("✓ 使用新配置加密成功\n")

	// 9. 演示相同密码产生不同哈希
	fmt.Println("9. 相同密码产生不同哈希（加盐效果）")
	samePassword := "samepassword"
	hash1, _ := cryptoInstance.HashPassword(samePassword)
	hash2, _ := cryptoInstance.HashPassword(samePassword)

	fmt.Printf("密码 1 哈希: %s\n", hash1)
	fmt.Printf("密码 2 哈希: %s\n", hash2)
	if hash1 != hash2 {
		fmt.Print("✓ 相同密码产生不同哈希（加盐生效）\n")
	} else {
		log.Fatal("错误：相同密码产生了相同的哈希")
	}

	// 10. 验证两个哈希都有效
	fmt.Println("10. 验证两个哈希都有效")
	if cryptoInstance.VerifyPassword(hash1, samePassword) == nil &&
		cryptoInstance.VerifyPassword(hash2, samePassword) == nil {
		fmt.Print("✓ 两个哈希都能验证原密码\n")
	} else {
		log.Fatal("错误：哈希验证失败")
	}

	fmt.Println("=== 示例运行成功 ===")
}
