package auth

import (
	"fmt"
	"kiro2api/config"
	"kiro2api/logger"
	"kiro2api/types"
	"strings"
	"sync"
	"time"
)

// TokenManager 简化的token管理器
type TokenManager struct {
	cache        *SimpleTokenCache
	configs      []AuthConfig
	mutex        sync.RWMutex
	lastRefresh  time.Time
	configOrder  []string // 配置顺序
	currentIndex int      // 当前使用的token索引
}

// SimpleTokenCache 简化的token缓存（纯数据结构，无锁）
// 所有并发访问由 TokenManager.mutex 统一管理
type SimpleTokenCache struct {
	tokens map[string]*CachedToken
	ttl    time.Duration
}

// CachedToken 缓存的token信息
type CachedToken struct {
	Token     types.TokenInfo
	UsageInfo *types.UsageLimits
	CachedAt  time.Time
	LastUsed  time.Time
	Available float64
}

// NewSimpleTokenCache 创建简单的token缓存
func NewSimpleTokenCache(ttl time.Duration) *SimpleTokenCache {
	return &SimpleTokenCache{
		tokens: make(map[string]*CachedToken),
		ttl:    ttl,
	}
}

// NewTokenManager 创建新的token管理器
func NewTokenManager(configs []AuthConfig) *TokenManager {
	// 生成配置顺序
	configOrder := generateConfigOrder(configs)

	logger.Info("TokenManager初始化（顺序耗尽策略）",
		logger.Int("config_count", len(configs)),
		logger.Int("config_order_count", len(configOrder)))

	return &TokenManager{
		cache:        NewSimpleTokenCache(config.TokenCacheTTL),
		configs:      configs,
		configOrder:  configOrder,
		currentIndex: 0,
	}
}

// getBestToken 获取最优可用token
// 统一锁管理：所有操作在单一锁保护下完成，避免多次加锁/解锁
func (tm *TokenManager) getBestToken() (types.TokenInfo, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// 检查是否需要刷新缓存（在锁内）
	if time.Since(tm.lastRefresh) > config.TokenCacheTTL {
		if err := tm.refreshCacheUnlocked(); err != nil {
			logger.Warn("刷新token缓存失败", logger.Err(err))
		}
	}

	// 选择最优token（内部方法，不加锁）
	bestToken := tm.selectBestTokenUnlocked()
	if bestToken == nil {
		return types.TokenInfo{}, fmt.Errorf("没有可用的token")
	}

	// 更新最后使用时间（在锁内，安全）
	bestToken.LastUsed = time.Now()
	if bestToken.Available > 0 {
		bestToken.Available--
	}

	return bestToken.Token, nil
}

// GetBestTokenWithUsage 获取最优可用token（包含使用信息）
// 统一锁管理：所有操作在单一锁保护下完成
func (tm *TokenManager) GetBestTokenWithUsage() (*types.TokenWithUsage, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// 检查是否需要刷新缓存（在锁内）
	if time.Since(tm.lastRefresh) > config.TokenCacheTTL {
		if err := tm.refreshCacheUnlocked(); err != nil {
			logger.Warn("刷新token缓存失败", logger.Err(err))
		}
	}

	// 选择最优token（内部方法，不加锁）
	bestToken := tm.selectBestTokenUnlocked()
	if bestToken == nil {
		return nil, fmt.Errorf("没有可用的token")
	}

	// 更新最后使用时间（在锁内，安全）
	bestToken.LastUsed = time.Now()
	if bestToken.Available > 0 {
		bestToken.Available--
	}

	// 构造 TokenWithUsage（不再检查使用限制）
	tokenWithUsage := &types.TokenWithUsage{
		TokenInfo:       bestToken.Token,
		UsageLimits:     nil,
		AvailableCount:  1,
		LastUsageCheck:  bestToken.LastUsed,
		IsUsageExceeded: false,
	}

	return tokenWithUsage, nil
}

// selectBestTokenUnlocked 按配置顺序选择token，优先耗尽当前token
// 策略：用完当前token后再切换到下一个（顺序耗尽策略）
// 内部方法：调用者必须持有 tm.mutex
func (tm *TokenManager) selectBestTokenUnlocked() *CachedToken {
	// 调用者已持有 tm.mutex，无需额外加锁

	// 如果没有配置顺序，降级到按map遍历顺序
	if len(tm.configOrder) == 0 {
		for key, cached := range tm.cache.tokens {
			if time.Since(cached.CachedAt) <= tm.cache.ttl && cached.IsUsable() {
				logger.Debug("顺序耗尽策略选择token（无顺序配置）",
					logger.String("selected_key", key),
					logger.Float64("available_count", cached.Available))
				return cached
			}
		}
		return nil
	}

	// 先检查当前token是否还可用（优先耗尽当前token）
	currentKey := tm.configOrder[tm.currentIndex]
	if cached, exists := tm.cache.tokens[currentKey]; exists {
		// 检查token是否过期
		if time.Since(cached.CachedAt) <= tm.cache.ttl {
			// 检查token是否可用
			if cached.IsUsable() {
				logger.Debug("顺序耗尽策略：继续使用当前token",
					logger.String("current_key", currentKey),
					logger.Int("index", tm.currentIndex),
					logger.Float64("available_count", cached.Available))
				return cached
			}
		}
	}

	// 当前token已耗尽或过期，查找下一个可用的token
	for attempts := 0; attempts < len(tm.configOrder); attempts++ {
		tm.currentIndex = (tm.currentIndex + 1) % len(tm.configOrder)
		nextKey := tm.configOrder[tm.currentIndex]

		if cached, exists := tm.cache.tokens[nextKey]; exists {
			// 检查token是否过期
			if time.Since(cached.CachedAt) > tm.cache.ttl {
				logger.Debug("token已过期，继续查找",
					logger.String("expired_key", nextKey))
				continue
			}

			// 检查token是否可用
			if cached.IsUsable() {
				logger.Info("顺序耗尽策略：切换到下一个token",
					logger.String("previous_key", currentKey),
					logger.String("new_key", nextKey),
					logger.Int("new_index", tm.currentIndex),
					logger.Float64("available_count", cached.Available))
				return cached
			}

			logger.Debug("token可用次数为0，继续查找",
				logger.String("exhausted_key", nextKey))
		}
	}

	// 所有token都不可用
	logger.Warn("所有token都不可用",
		logger.Int("total_count", len(tm.configOrder)))

	return nil
}

// refreshCacheUnlocked 刷新token缓存
// 内部方法：调用者必须持有 tm.mutex
func (tm *TokenManager) refreshCacheUnlocked() error {
	logger.Debug("开始刷新token缓存")

	for i, cfg := range tm.configs {
		if cfg.Disabled {
			continue
		}

		// 如果配置中有AccessToken，优先使用（刚从device flow获取的token）
		var token types.TokenInfo
		if cfg.AccessToken != "" {
			logger.Debug("使用配置中的AccessToken（刚获取的token）",
				logger.Int("config_index", i),
				logger.String("auth_type", cfg.AuthType))
			token = types.Token{
				AccessToken:  cfg.AccessToken,
				RefreshToken: cfg.RefreshToken,
				ExpiresAt:    time.Now().Add(24 * time.Hour),
			}
		} else {
			// 否则尝试刷新token
			var err error
			token, err = tm.refreshSingleToken(cfg)
			if err != nil {
				logger.Warn("刷新token失败，跳过此配置",
					logger.Int("config_index", i),
					logger.String("auth_type", cfg.AuthType),
					logger.Err(err))
				continue
			}
		}

			// 更新缓存（不检查使用限制，用完就换下一个）
		cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, i)
		tm.cache.tokens[cacheKey] = &CachedToken{
			Token:     token,
			UsageInfo: nil,
			CachedAt:  time.Now(),
			Available: 1, // 简单计数：1表示可用，0表示已耗尽
		}

		logger.Debug("token缓存更新",
			logger.String("cache_key", cacheKey))
	}

	tm.lastRefresh = time.Now()
	return nil
}

// IsUsable 检查缓存的token是否可用
func (ct *CachedToken) IsUsable() bool {
	// 检查token是否过期
	if time.Now().After(ct.Token.ExpiresAt) {
		return false
	}

	// 检查可用次数
	return ct.Available > 0
}

// GetCurrentTokenCacheKey 获取当前使用的token的缓存键
func (tm *TokenManager) GetCurrentTokenCacheKey() string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if len(tm.configOrder) == 0 {
		return ""
	}

	if tm.currentIndex >= len(tm.configOrder) {
		return ""
	}

	return tm.configOrder[tm.currentIndex]
}

// MarkTokenInvalid 标记指定的token为失效，强制切换到下一个可用token
// 当收到403错误时调用此方法
func (tm *TokenManager) MarkTokenInvalid(cacheKey string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if cached, exists := tm.cache.tokens[cacheKey]; exists {
		// 将可用次数设置为0，标记为失效
		cached.Available = 0
		logger.Warn("Token标记为失效",
			logger.String("cache_key", cacheKey),
			logger.String("token_preview", createTokenPreview(cached.Token.AccessToken)))

		// 同时在内存中禁用对应的configs条目，防止下次refreshCache重新加载此token
		for i, key := range tm.configOrder {
			if key == cacheKey && i < len(tm.configs) {
				tm.configs[i].Disabled = true
				logger.Info("已在内存中禁用对应的configs条目",
					logger.Int("config_index", i),
					logger.String("cache_key", cacheKey))
				break
			}
		}

		// 尝试持久化到配置文件
		if err := DisableTokenInConfig(cached.Token.RefreshToken); err != nil {
			logger.Warn("无法持久化token失效状态",
				logger.String("cache_key", cacheKey),
				logger.Err(err))
			// 注意：即使持久化失败，仍继续标记为失效（内存中）
		}

		// 强制重置currentIndex，下次会选择下一个可用的token
		// 找到当前失效token在configOrder中的位置
		for i, key := range tm.configOrder {
			if key == cacheKey {
				// 设置currentIndex为当前位置，selectBestTokenUnlocked会自动跳过并选择下一个
				tm.currentIndex = i
				logger.Info("重置Token选择索引以切换到下一个token",
					logger.Int("reset_index", tm.currentIndex),
					logger.String("invalid_key", cacheKey))
				break
			}
		}
		return nil
	}

	logger.Warn("无法标记Token为失效：缓存键不存在",
		logger.String("cache_key", cacheKey))
	return fmt.Errorf("token cache key not found: %s", cacheKey)
}

// *** 已删除 set 和 updateLastUsed 方法 ***
// SimpleTokenCache 现在是纯数据结构，所有访问由 TokenManager.mutex 保护
// set 操作：直接通过 tm.cache.tokens[key] = value 完成
// updateLastUsed 操作：已合并到 getBestToken 方法中


// generateConfigOrder 生成token配置的顺序
func generateConfigOrder(configs []AuthConfig) []string {
	var order []string

	for i := range configs {
		// 使用索引生成cache key，与refreshCache中的逻辑保持一致
		cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, i)
		order = append(order, cacheKey)
	}

	logger.Debug("生成配置顺序",
		logger.Int("config_count", len(configs)),
		logger.Any("order", order))

	return order
}

// createTokenPreview 创建token预览显示格式 (***+后10位)
func createTokenPreview(token string) string {
	if len(token) <= 10 {
		return strings.Repeat("*", len(token))
	}
	suffix := token[len(token)-10:]
	return "***" + suffix
}
