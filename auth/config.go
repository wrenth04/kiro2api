package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"kiro2api/logger"
)

// AuthConfig 简化的认证配置
type AuthConfig struct {
	AuthType     string `json:"auth"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
}

// 认证方法常量
const (
	AuthMethodSocial = "Social"
	AuthMethodIdC    = "IdC"
)

// configFilePath 全局变量，存储配置文件路径（用于更新失效token）
var (
	configFilePath string
	configMutex    sync.RWMutex
)

// loadConfigs 从环境变量加载配置
func loadConfigs() ([]AuthConfig, error) {
	// 检测并警告弃用的环境变量
	deprecatedVars := []string{
		"REFRESH_TOKEN",
		"AWS_REFRESHTOKEN",
		"IDC_REFRESH_TOKEN",
		"BULK_REFRESH_TOKENS",
	}

	for _, envVar := range deprecatedVars {
		if os.Getenv(envVar) != "" {
			logger.Warn("检测到已弃用的环境变量",
				logger.String("变量名", envVar),
				logger.String("迁移说明", "请迁移到KIRO_AUTH_TOKEN的JSON格式"))
			logger.Warn("迁移示例",
				logger.String("新格式", `KIRO_AUTH_TOKEN='[{"auth":"Social","refreshToken":"your_token"}]'`))
		}
	}

	// 只支持KIRO_AUTH_TOKEN的JSON格式（支持文件路径或JSON字符串）
	jsonData := os.Getenv("KIRO_AUTH_TOKEN")
	if jsonData == "" {
		return nil, fmt.Errorf("未找到KIRO_AUTH_TOKEN环境变量\n" +
			"请设置: KIRO_AUTH_TOKEN='[{\"auth\":\"Social\",\"refreshToken\":\"your_token\"}]'\n" +
			"或设置为配置文件路径: KIRO_AUTH_TOKEN=/path/to/config.json\n" +
			"支持的认证方式: Social, IdC\n" +
			"详细配置请参考: .env.example")
	}

	// 优先尝试从文件加载，失败后再作为JSON字符串处理
	var configData string
	if fileInfo, err := os.Stat(jsonData); err == nil && !fileInfo.IsDir() {
		// 是文件，读取文件内容
		content, err := os.ReadFile(jsonData)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件失败: %w\n配置文件路径: %s", err, jsonData)
		}
		configData = string(content)
		// 记录配置文件路径
		configMutex.Lock()
		configFilePath = jsonData
		configMutex.Unlock()
		logger.Info("从文件加载认证配置", logger.String("文件路径", jsonData))
	} else {
		// 不是文件或文件不存在，作为JSON字符串处理
		configData = jsonData
		logger.Debug("从环境变量加载JSON配置")
	}

	// 解析JSON配置
	configs, err := parseJSONConfig(configData)
	if err != nil {
		return nil, fmt.Errorf("解析KIRO_AUTH_TOKEN失败: %w\n"+
			"请检查JSON格式是否正确\n"+
			"示例: KIRO_AUTH_TOKEN='[{\"auth\":\"Social\",\"refreshToken\":\"token1\"}]'", err)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("KIRO_AUTH_TOKEN配置为空，请至少提供一个有效的认证配置")
	}

	validConfigs := processConfigs(configs)
	if len(validConfigs) == 0 {
		return nil, fmt.Errorf("没有有效的认证配置\n" +
			"请检查: \n" +
			"1. Social认证需要refreshToken字段\n" +
			"2. IdC认证需要refreshToken、clientId、clientSecret字段")
	}

	logger.Info("成功加载认证配置",
		logger.Int("总配置数", len(configs)),
		logger.Int("有效配置数", len(validConfigs)))

	return validConfigs, nil
}

// GetConfigs 公开的配置获取函数，供其他包调用
func GetConfigs() ([]AuthConfig, error) {
	return loadConfigs()
}

// parseJSONConfig 解析JSON配置字符串
func parseJSONConfig(jsonData string) ([]AuthConfig, error) {
	var configs []AuthConfig

	// 尝试解析为数组
	if err := json.Unmarshal([]byte(jsonData), &configs); err != nil {
		// 尝试解析为单个对象
		var single AuthConfig
		if err := json.Unmarshal([]byte(jsonData), &single); err != nil {
			return nil, fmt.Errorf("JSON格式无效: %w", err)
		}
		configs = []AuthConfig{single}
	}

	return configs, nil
}

// processConfigs 处理和验证配置
func processConfigs(configs []AuthConfig) []AuthConfig {
	var validConfigs []AuthConfig

	for i, config := range configs {
		// 验证必要字段
		if config.RefreshToken == "" {
			continue
		}

		// 设置默认认证类型
		if config.AuthType == "" {
			config.AuthType = AuthMethodSocial
		}

		// 验证IdC认证的必要字段
		if config.AuthType == AuthMethodIdC {
			if config.ClientID == "" || config.ClientSecret == "" {
				continue
			}
		}

		// 跳过禁用的配置
		if config.Disabled {
			continue
		}

		validConfigs = append(validConfigs, config)
		_ = i // 避免未使用变量警告
	}

	return validConfigs
}

// DisableTokenInConfig 在配置文件中标记指定的token为禁用
// refreshToken: 要禁用的token的refreshToken值
func DisableTokenInConfig(refreshToken string) error {
	configMutex.RLock()
	filePath := configFilePath
	configMutex.RUnlock()

	// 如果没有配置文件路径，无法保存
	if filePath == "" {
		logger.Warn("无法持久化token失效状态：未从文件加载配置",
			logger.String("reason", "配置来自环境变量JSON字符串"))
		return fmt.Errorf("configuration not loaded from file, cannot persist changes")
	}

	// 读取现有配置
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("读取配置文件失败",
			logger.String("file_path", filePath),
			logger.Err(err))
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置
	var configs []AuthConfig
	if err := json.Unmarshal(content, &configs); err != nil {
		// 尝试解析为单个对象
		var single AuthConfig
		if err := json.Unmarshal(content, &single); err != nil {
			logger.Error("解析配置文件JSON失败",
				logger.String("file_path", filePath),
				logger.Err(err))
			return fmt.Errorf("failed to parse config file: %w", err)
		}
		configs = []AuthConfig{single}
	}

	// 查找并禁用对应的token
	found := false
	for i := range configs {
		if configs[i].RefreshToken == refreshToken {
			configs[i].Disabled = true
			found = true
			logger.Info("标记token为禁用",
				logger.Int("config_index", i),
				logger.String("refresh_token_preview", createTokenPreviewConfig(refreshToken)))
			break
		}
	}

	if !found {
		logger.Warn("无法在配置中找到指定的token",
			logger.String("refresh_token_preview", createTokenPreviewConfig(refreshToken)))
		return fmt.Errorf("token not found in config file")
	}

	// 写回配置文件
	updatedContent, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		logger.Error("序列化配置失败",
			logger.Err(err))
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, updatedContent, 0600); err != nil {
		logger.Error("写入配置文件失败",
			logger.String("file_path", filePath),
			logger.Err(err))
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.Info("Token禁用状态已持久化到配置文件",
		logger.String("file_path", filePath))
	return nil
}

// createTokenPreviewConfig 为config.go中使用创建token预览
func createTokenPreviewConfig(token string) string {
	if len(token) <= 10 {
		return "***" + token[len(token):]
	}
	return "***" + token[len(token)-10:]
}
