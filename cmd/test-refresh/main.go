package main

import (
	"fmt"
	"kiro2api/auth"
	"kiro2api/logger"
	"strings"
)

func main() {
	logger.Reinitialize()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  Token 刷新测试")
	fmt.Println(strings.Repeat("=", 60))

	// 测试 1: 加载配置
	fmt.Println("\n📝 测试 1: 加载配置")
	fmt.Println(strings.Repeat("-", 60))

	configs, err := auth.GetConfigs()
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 成功加载 %d 个配置\n", len(configs))
	for i, cfg := range configs {
		tokenPreview := cfg.RefreshToken
		if len(tokenPreview) > 20 {
			tokenPreview = tokenPreview[:20] + "..."
		}
		fmt.Printf("   [%d] auth=%s refreshToken=%s disabled=%v\n",
			i, cfg.AuthType, tokenPreview, cfg.Disabled)
	}

	// 测试 2: 创建 TokenManager
	fmt.Println("\n📝 测试 2: 创建 TokenManager")
	fmt.Println(strings.Repeat("-", 60))

	tm := auth.NewTokenManager(configs)
	fmt.Println("✅ TokenManager 创建完成")

	// 测试 3: 获取 token（触发缓存刷新）
	fmt.Println("\n📝 测试 3: 获取 token（第一次 - 触发缓存刷新）")
	fmt.Println(strings.Repeat("-", 60))

	token1, err := tm.GetBestTokenWithUsage()
	if err != nil {
		fmt.Printf("❌ 获取 token 失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 成功获取 token\n")
	tokenPreview := token1.TokenInfo.AccessToken
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	fmt.Printf("   AccessToken: %s\n", tokenPreview)
	fmt.Printf("   AvailableCount: %.1f\n", token1.AvailableCount)
	fmt.Printf("   IsUsageExceeded: %v\n", token1.IsUsageExceeded)
	if token1.UsageLimits != nil {
		fmt.Printf("   ✓ 使用限制信息已获取\n")
	}

	// 测试 4: 第二次获取 token（使用缓存）
	fmt.Println("\n📝 测试 4: 获取 token（第二次 - 使用缓存）")
	fmt.Println(strings.Repeat("-", 60))

	token2, err := tm.GetBestTokenWithUsage()
	if err != nil {
		fmt.Printf("❌ 获取 token 失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 成功获取 token\n")
	fmt.Printf("   AvailableCount: %.1f (之前: %.1f)\n", token2.AvailableCount, token1.AvailableCount)
	fmt.Printf("   IsUsageExceeded: %v\n", token2.IsUsageExceeded)

	// 验证可用次数是否正确递减
	if token2.AvailableCount < token1.AvailableCount {
		fmt.Printf("   ✅ 可用次数正确递减\n")
	} else {
		fmt.Printf("   ⚠️  可用次数未递减\n")
	}

	// 测试 5: 并发获取 token
	fmt.Println("\n📝 测试 5: 并发获取 token（并发安全性）")
	fmt.Println(strings.Repeat("-", 60))

	done := make(chan bool)
	results := make(chan float64, 10)

	for i := 0; i < 10; i++ {
		go func() {
			token, err := tm.GetBestTokenWithUsage()
			if err == nil {
				results <- token.AvailableCount
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	close(results)

	fmt.Println("✅ 10 个并发请求完成")
	count := 0
	for range results {
		count++
	}
	fmt.Printf("   成功获取 %d 个 token\n", count)

	// 总结
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  测试完成")
	fmt.Println(strings.Repeat("=", 60))
}

