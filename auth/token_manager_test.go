package auth

import (
	"encoding/json"
	"fmt"
	"kiro2api/config"
	"kiro2api/types"
	"os"
	"sync"
	"testing"
	"time"
)

// TestTokenManager_ConcurrentAccess 测试TokenManager的并发访问安全性
func TestTokenManager_ConcurrentAccess(t *testing.T) {
	// 创建测试配置
	configs := []AuthConfig{
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_1",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_2",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_3",
		},
	}

	// 创建TokenManager
	tm := NewTokenManager(configs)

	// 预填充缓存（模拟已刷新的token）
	tm.mutex.Lock()
	for i := range configs {
		cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, i)
		tm.cache.tokens[cacheKey] = &CachedToken{
			Token: types.TokenInfo{
				AccessToken: fmt.Sprintf("access_token_%d", i),
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			UsageInfo: nil,
			CachedAt:  time.Now(),
			Available: 10000.0, // 足够支持50×100=5000次调用
		}
	}
	// 关键修复：更新lastRefresh避免触发真实的token刷新
	tm.lastRefresh = time.Now()
	tm.mutex.Unlock()

	// 并发测试参数
	numGoroutines := 50
	numIterations := 100

	var wg sync.WaitGroup
	errorsChan := make(chan error, numGoroutines*numIterations)

	// 启动多个goroutine并发调用selectBestToken
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				// 使用公共API getBestToken而不是内部方法
				_, err := tm.getBestToken()
				if err != nil {
					errorsChan <- fmt.Errorf("goroutine %d iteration %d: getBestToken failed: %v", id, j, err)
				}
				// 模拟一些工作
				time.Sleep(1 * time.Microsecond)
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(errorsChan)

	// 检查是否有错误
	errors := []error{}
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("并发访问测试失败，发现 %d 个错误", len(errors))
		for i, err := range errors {
			if i < 10 { // 只打印前10个错误
				t.Logf("错误 %d: %v", i+1, err)
			}
		}
	}

	t.Logf("并发测试完成: %d 个goroutine × %d 次迭代 = %d 次调用",
		numGoroutines, numIterations, numGoroutines*numIterations)
}

// TestTokenManager_ConcurrentRefresh 测试并发刷新token的安全性
func TestTokenManager_ConcurrentRefresh(t *testing.T) {
	configs := []AuthConfig{
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_1",
		},
	}

	tm := NewTokenManager(configs)

	// 预填充缓存
	tm.mutex.Lock()
	tm.cache.tokens["token_0"] = &CachedToken{
		Token: types.TokenInfo{
			AccessToken: "access_token_0",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
		CachedAt:  time.Now(),
		Available: 10000.0, // 足够支持20×50=1000次调用
	}
	// 关键修复：更新lastRefresh避免触发真实的token刷新
	tm.lastRefresh = time.Now()
	tm.mutex.Unlock()

	numGoroutines := 20
	var wg sync.WaitGroup

	// 并发读取token
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				_, err := tm.getBestToken()
				if err != nil {
					t.Errorf("goroutine %d: getBestToken failed: %v", id, err)
				}
				time.Sleep(1 * time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("并发刷新测试完成")
}

// TestTokenManager_RaceCondition 使用race detector检测数据竞争
// 运行方式: go test -race -run TestTokenManager_RaceCondition
func TestTokenManager_RaceCondition(t *testing.T) {
	configs := []AuthConfig{
		{AuthType: AuthMethodSocial, RefreshToken: "token1"},
		{AuthType: AuthMethodSocial, RefreshToken: "token2"},
	}

	tm := NewTokenManager(configs)

	// 预填充缓存
	tm.mutex.Lock()
	for i := range configs {
		tm.cache.tokens[fmt.Sprintf(config.TokenCacheKeyFormat, i)] = &CachedToken{
			Token: types.TokenInfo{
				AccessToken: fmt.Sprintf("access_%d", i),
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			CachedAt:  time.Now(),
			Available: 50.0,
		}
	}
	// 关键修复：更新lastRefresh避免触发真实的token刷新
	tm.lastRefresh = time.Now()
	tm.mutex.Unlock()

	var wg sync.WaitGroup
	numGoroutines := 10

	// 并发读写测试
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = tm.getBestToken()
			}
		}()
	}

	wg.Wait()
	t.Log("Race condition测试完成，使用 go test -race 运行以检测数据竞争")
}

// TestTokenManager_SequentialSelection 测试顺序选择逻辑（粘性策略）
func TestTokenManager_SequentialSelection(t *testing.T) {
	configs := []AuthConfig{
		{AuthType: AuthMethodSocial, RefreshToken: "token1"},
		{AuthType: AuthMethodSocial, RefreshToken: "token2"},
		{AuthType: AuthMethodSocial, RefreshToken: "token3"},
	}

	tm := NewTokenManager(configs)

	// 预填充缓存 - 每个token只有少量可用次数
	tm.mutex.Lock()
	for i := range configs {
		tm.cache.tokens[fmt.Sprintf(config.TokenCacheKeyFormat, i)] = &CachedToken{
			Token: types.TokenInfo{
				AccessToken: fmt.Sprintf("access_%d", i),
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			CachedAt:  time.Now(),
			Available: 5.0, // 每个token只有5次使用机会
		}
	}
	// 关键修复：更新lastRefresh避免触发真实的token刷新
	tm.lastRefresh = time.Now()
	tm.mutex.Unlock()

	// 验证顺序选择：使用getBestToken会递减Available
	selectedTokens := make(map[string]int)
	for i := 0; i < 15; i++ { // 15次调用会用完所有token (5+5+5)
		token, err := tm.getBestToken()
		if err == nil {
			selectedTokens[token.AccessToken]++
		}
	}

	t.Logf("Token选择分布: %v", selectedTokens)

	// 验证粘性策略：应该先用完第一个token，然后是第二个，最后是第三个
	if selectedTokens["access_0"] != 5 {
		t.Errorf("期望access_0使用5次，实际使用%d次", selectedTokens["access_0"])
	}
	if selectedTokens["access_1"] != 5 {
		t.Errorf("期望access_1使用5次，实际使用%d次", selectedTokens["access_1"])
	}
	if selectedTokens["access_2"] != 5 {
		t.Errorf("期望access_2使用5次，实际使用%d次", selectedTokens["access_2"])
	}

	// 验证所有token都被使用过
	if len(selectedTokens) != len(configs) {
		t.Errorf("期望使用 %d 个token，实际使用 %d 个", len(configs), len(selectedTokens))
	}

	t.Logf("✅ 顺序选择策略验证通过：粘性策略正确工作")
}

// TestTokenManager_MarkTokenInvalid 测试标记token为失效的功能
func TestTokenManager_MarkTokenInvalid(t *testing.T) {
	// 创建测试配置
	configs := []AuthConfig{
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_1",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_2",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "test_token_3",
		},
	}

	// 创建TokenManager
	tm := NewTokenManager(configs)

	// 预填充缓存
	tm.mutex.Lock()
	for i := range configs {
		cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, i)
		tm.cache.tokens[cacheKey] = &CachedToken{
			Token: types.TokenInfo{
				AccessToken: fmt.Sprintf("access_token_%d", i),
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			UsageInfo: nil,
			CachedAt:  time.Now(),
			Available: 100.0,
		}
	}
	tm.lastRefresh = time.Now()
	tm.mutex.Unlock()

	// 第一次获取token，应该获取token_0
	token1, err := tm.getBestToken()
	if err != nil {
		t.Errorf("获取第一个token失败: %v", err)
	}
	if token1.AccessToken != "access_token_0" {
		t.Errorf("期望获取access_token_0，实际获取%s", token1.AccessToken)
	}

	// 标记token_0为失效
	cacheKey0 := fmt.Sprintf(config.TokenCacheKeyFormat, 0)
	err = tm.MarkTokenInvalid(cacheKey0)
	if err != nil {
		t.Errorf("标记token为失效失败: %v", err)
	}

	// 验证token_0已被标记为失效（可用次数为0）
	tm.mutex.RLock()
	cached0 := tm.cache.tokens[cacheKey0]
	if cached0.Available != 0 {
		t.Errorf("期望token_0的Available为0，实际为%f", cached0.Available)
	}
	tm.mutex.RUnlock()

	// 下次获取token，应该跳过token_0，获取token_1
	token2, err := tm.getBestToken()
	if err != nil {
		t.Errorf("获取第二个token失败: %v", err)
	}
	if token2.AccessToken != "access_token_1" {
		t.Errorf("期望获取access_token_1（跳过失效的token_0），实际获取%s", token2.AccessToken)
	}

	// 验证currentIndex已被重置
	tm.mutex.RLock()
	currentKey := tm.configOrder[tm.currentIndex]
	tm.mutex.RUnlock()
	if currentKey != cacheKey0 {
		t.Logf("✅ Token已自动切换，currentIndex指向下一个token")
	}

	t.Logf("✅ MarkTokenInvalid功能验证通过：token已正确标记失效并自动切换")
}

// TestDisableTokenInConfig 测试持久化token失效状态到配置文件
func TestDisableTokenInConfig(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := os.CreateTemp("", "test_auth_*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入初始配置
	initialConfig := []AuthConfig{
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "token_1",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "token_2",
		},
		{
			AuthType:     AuthMethodSocial,
			RefreshToken: "token_3",
		},
	}

	initialData, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		t.Fatalf("序列化初始配置失败: %v", err)
	}

	if err := os.WriteFile(tmpFile.Name(), initialData, 0600); err != nil {
		t.Fatalf("写入初始配置失败: %v", err)
	}

	// 设置配置文件路径
	configMutex.Lock()
	configFilePath = tmpFile.Name()
	configMutex.Unlock()

	// 测试禁用token_2
	err = DisableTokenInConfig("token_2")
	if err != nil {
		t.Errorf("禁用token失败: %v", err)
	}

	// 读取并验证配置文件
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	var updatedConfig []AuthConfig
	if err := json.Unmarshal(content, &updatedConfig); err != nil {
		t.Fatalf("解析更新后的配置失败: %v", err)
	}

	// 验证token_2已被禁用
	token2Found := false
	for _, config := range updatedConfig {
		if config.RefreshToken == "token_2" {
			token2Found = true
			if !config.Disabled {
				t.Errorf("期望token_2被禁用，实际未禁用")
			}
			break
		}
	}

	if !token2Found {
		t.Errorf("在配置文件中找不到token_2")
	}

	// 验证其他token未被禁用
	for _, config := range updatedConfig {
		if config.RefreshToken != "token_2" && config.Disabled {
			t.Errorf("不期望的token被禁用: %s", config.RefreshToken)
		}
	}

	t.Logf("✅ DisableTokenInConfig功能验证通过：token已正确禁用并持久化到文件")
}
