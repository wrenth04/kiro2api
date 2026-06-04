package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiro2api/auth"
	"kiro2api/logger"
	"kiro2api/server"

	"github.com/joho/godotenv"
)

func main() {
	// 自动加载.env文件（如果存在）
	// godotenv.Load 不会覆盖已设置的环境变量
	_ = godotenv.Load()

	// 重新初始化logger以使用.env文件中的配置
	logger.Reinitialize()

	// 检查子命令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "get-token":
			runGetTokenCommand()
			return
		case "setup":
			runSetupCommand()
			return
		case "--help", "-h", "help":
			printHelp()
			return
		}
	}

	// 显示当前日志级别设置（仅在DEBUG级别时显示详细信息）
	logger.Debug("日志系统初始化完成",
		logger.String("config_level", os.Getenv("LOG_LEVEL")),
		logger.String("config_file", os.Getenv("LOG_FILE")))

	// 🚀 创建AuthService实例（使用依赖注入）
	logger.Info("正在创建AuthService...")
	authService, err := auth.NewAuthService()
	if err != nil {
		logger.Error("AuthService创建失败", logger.Err(err))
		logger.Error("请检查token配置后重新启动服务器")
		os.Exit(1)
	}

	port := "8080" // 默认端口
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	// 从环境变量获取端口，覆盖命令行参数
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	// 从环境变量获取客户端认证token（必需，无默认值）
	clientToken := os.Getenv("KIRO_CLIENT_TOKEN")
	if clientToken == "" {
		logger.Error("致命错误: 未设置 KIRO_CLIENT_TOKEN 环境变量")
		logger.Error("请在 .env 文件中设置强密码，例如: KIRO_CLIENT_TOKEN=your-secure-random-password")
		logger.Error("安全提示: 请使用至少32字符的随机字符串")
		os.Exit(1)
	}

	server.StartServer(port, clientToken, authService)
}

func runGetTokenCommand() {
	fs := flag.NewFlagSet("get-token", flag.ExitOnError)
	output := fs.String("output", "", "输出文件路径（默认：打印到标准输出）")
	authType := fs.String("type", "Social", "认证类型: Social 或 IdC")

	fs.Parse(os.Args[2:])

	logger.Debug("日志系统初始化完成")

	// 执行设备授权流程
	tokenResult, err := auth.GetDeviceFlowToken(*authType)
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: %v\n", err)
		os.Exit(1)
	}

	// 构建新token配置
	newTokenConfig := auth.AuthConfig{
		AuthType:     *authType,
		AccessToken:  tokenResult.AccessToken,
		RefreshToken: tokenResult.RefreshToken,
		Disabled:     false,
	}

	// 确定输出文件路径
	outputPath := *output
	if outputPath == "" {
		// 如果未指定输出路径，检查环境变量 KIRO_AUTH_TOKEN
		envPath := os.Getenv("KIRO_AUTH_TOKEN")
		if envPath != "" {
			// 检查是否为文件路径（而非JSON字符串）
			if fileInfo, err := os.Stat(envPath); err == nil && !fileInfo.IsDir() {
				outputPath = envPath
				fmt.Printf("📂 检测到环境变量 KIRO_AUTH_TOKEN 指向文件: %s\n", envPath)
			}
		}
	}

	// 准备配置
	var config []auth.AuthConfig

	// 如果输出路径指向现有文件，则追加到现有配置
	if outputPath != "" {
		if fileInfo, err := os.Stat(outputPath); err == nil && !fileInfo.IsDir() {
			// 文件存在，读取现有配置
			existingData, err := os.ReadFile(outputPath)
			if err == nil {
				var existingConfigs []auth.AuthConfig
				if err := json.Unmarshal(existingData, &existingConfigs); err == nil {
					config = existingConfigs
					fmt.Printf("✓ 读取现有配置 (%d个token)\n", len(config))
					fmt.Println("📝 将新token追加到配置列表")
				} else {
					fmt.Printf("⚠️  警告: 无法解析现有配置文件，将创建新列表\n")
					config = []auth.AuthConfig{newTokenConfig}
				}
			} else {
				fmt.Printf("⚠️  警告: 无法读取现有配置文件，将创建新列表\n")
				config = []auth.AuthConfig{newTokenConfig}
			}
		} else {
			// 文件不存在，创建新配置
			config = []auth.AuthConfig{newTokenConfig}
		}
	} else {
		// 未指定输出路径，只创建新配置（打印到标准输出）
		config = []auth.AuthConfig{newTokenConfig}
	}

	// 如果是追加操作且配置不只有新token，添加新token
	if outputPath != "" && len(config) > 0 && !isNewConfig(config, newTokenConfig) {
		config = append(config, newTokenConfig)
		fmt.Printf("✓ 已将新token追加到配置列表 (现在共%d个token)\n", len(config))
	} else if outputPath == "" {
		// 打印到标准输出时只包含新token
		config = []auth.AuthConfig{newTokenConfig}
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error("序列化配置失败", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: 无法序列化配置\n")
		os.Exit(1)
	}

	// 输出或保存到文件
	if outputPath != "" {
		// 保存到文件（权限: 0600）
		if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
			logger.Error("写入文件失败", logger.Err(err))
			fmt.Fprintf(os.Stderr, "❌ 错误: 无法写入文件 %s: %v\n", outputPath, err)
			os.Exit(1)
		}
		fmt.Printf("✓ 配置已保存到: %s\n", outputPath)
		fmt.Println("\n接下来，你可以这样启动服务器:")
		fmt.Printf("  KIRO_AUTH_TOKEN=%s ./kiro2api\n", outputPath)
	} else {
		// 打印到标准输出
		fmt.Println("⚠️  警告: Token包含敏感信息，请妥善保管")
		fmt.Println("\n配置内容:")
		fmt.Println(string(jsonData))
		fmt.Println("\n你可以将上述内容保存到文件，例如:")
		fmt.Println("  ./kiro2api get-token --output auth_config.json")
		fmt.Println("\n或者自动追加到环境变量指定的文件:")
		fmt.Println("  KIRO_AUTH_TOKEN=/path/to/config.json ./kiro2api get-token")
	}
}

// isNewConfig 检查是否是全新配置（只包含一个token且为新token）
func isNewConfig(config []auth.AuthConfig, newToken auth.AuthConfig) bool {
	return len(config) == 1 &&
		config[0].RefreshToken == newToken.RefreshToken &&
		config[0].AuthType == newToken.AuthType
}

func runSetupCommand() {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	authConfigFile := fs.String("auth", "", "认证配置文件路径（必需）")
	clientToken := fs.String("client-token", "", "客户端认证token（可选，默认生成随机）")
	port := fs.String("port", "8080", "服务端口（默认: 8080）")
	envFile := fs.String("env", ".env.local", "输出.env文件路径（默认: .env.local）")

	fs.Parse(os.Args[2:])

	// 验证认证配置文件
	if *authConfigFile == "" {
		fmt.Fprintf(os.Stderr, "❌ 错误: 必须指定 --auth 参数\n")
		fmt.Fprintf(os.Stderr, "用法: ./kiro2api setup --auth auth.json\n")
		os.Exit(1)
	}

	// 检查认证配置文件是否存在
	_, err := os.Stat(*authConfigFile)
	if err != nil {
		logger.Error("认证配置文件不存在", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: 找不到文件 %s\n", *authConfigFile)
		os.Exit(1)
	}

	// 生成或使用提供的客户端token
	finalClientToken := *clientToken
	if finalClientToken == "" {
		finalClientToken = generateRandomToken(32)
		fmt.Printf("✓ 已生成随机客户端token\n")
	}

	// 构建.env文件内容
	absAuthPath, _ := filepath.Abs(*authConfigFile)
	envContent := fmt.Sprintf(`# kiro2api 配置文件（自动生成）
# 生成时间: %s

# 服务端口
PORT=%s

# 客户端认证token（用于保护API）
KIRO_CLIENT_TOKEN=%s

# 认证配置文件路径
KIRO_AUTH_TOKEN=%s

# 日志配置
LOG_LEVEL=info
LOG_FORMAT=json

# 工具描述长度限制（字符数）
TOOL_DESCRIPTION_MAX_LENGTH=500
`, time.Now().Format("2006-01-02 15:04:05"), *port, finalClientToken, absAuthPath)

	// 写入.env文件
	if err := os.WriteFile(*envFile, []byte(envContent), 0600); err != nil {
		logger.Error("写入.env文件失败", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: 无法写入文件 %s: %v\n", *envFile, err)
		os.Exit(1)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  ✓ 配置完成！")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("\n📝 配置文件: %s\n", *envFile)
	fmt.Printf("🔐 客户端token: %s\n", finalClientToken)
	fmt.Printf("🌐 服务端口: %s\n", *port)
	fmt.Printf("📂 认证配置: %s\n", absAuthPath)

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("  启动服务器:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("\n  source %s && ./kiro2api\n\n", *envFile)

	fmt.Println("或者：")
	fmt.Printf("\n  PORT=%s KIRO_CLIENT_TOKEN=%s KIRO_AUTH_TOKEN=%s ./kiro2api\n\n",
		*port, finalClientToken, absAuthPath)

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("⚠️  请妥善保管客户端token，它用于保护API接口！")
	fmt.Println(strings.Repeat("=", 60))
}

func printHelp() {
	fmt.Println("kiro2api - AWS CodeWhisperer API 代理服务器")
	fmt.Println("\n用法:")
	fmt.Println("  ./kiro2api [PORT]              启动服务器 (默认端口: 8080)")
	fmt.Println("  ./kiro2api get-token           获取AWS SSO token (交互式)")
	fmt.Println("  ./kiro2api get-token [OPTIONS] 获取token并保存配置")
	fmt.Println("  ./kiro2api setup [OPTIONS]     自动设定环境配置")
	fmt.Println("\n子命令:")
	fmt.Println("  get-token                      启动AWS SSO设备授权流程")
	fmt.Println("                                 交互式显示登录链接和验证码")
	fmt.Println("  setup                          使用auth.json自动配置")
	fmt.Println("\n选项:")
	fmt.Println("  get-token 选项:")
	fmt.Println("    -output string               输出文件路径（默认：打印到终端）")
	fmt.Println("    -type string                 认证类型: Social 或 IdC (默认: Social)")
	fmt.Println("\n  setup 选项:")
	fmt.Println("    -auth string                 认证配置文件路径 (必需)")
	fmt.Println("    -client-token string         客户端认证token (可选，默认生成随机)")
	fmt.Println("    -port string                 服务端口 (默认: 8080)")
	fmt.Println("    -env string                  输出.env文件路径 (默认: .env.local)")
	fmt.Println("\n示例:")
	fmt.Println("  # 1. 获取token")
	fmt.Println("  ./kiro2api get-token --output auth.json")
	fmt.Println("\n  # 2. 自动设定配置")
	fmt.Println("  ./kiro2api setup --auth auth.json")
	fmt.Println("\n  # 3. 自定义设定")
	fmt.Println("  ./kiro2api setup --auth auth.json --port 9000 --env .env.prod")
	fmt.Println("\n  # 4. 启动服务器")
	fmt.Println("  source .env.local && ./kiro2api")
	fmt.Println("\n环境变量:")
	fmt.Println("  PORT                  服务端口 (默认: 8080)")
	fmt.Println("  KIRO_CLIENT_TOKEN     API认证密钥 (必需)")
	fmt.Println("  KIRO_AUTH_TOKEN       AWS认证配置 (必需)")
	fmt.Println("  LOG_LEVEL             日志级别 (debug/info/warn/error)")
	fmt.Println("  LOG_FORMAT            日志格式 (text/json)")
}

func generateRandomToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		logger.Error("生成随机token失败", logger.Err(err))
		os.Exit(1)
	}
	return base64.URLEncoding.EncodeToString(b)[:length]
}

