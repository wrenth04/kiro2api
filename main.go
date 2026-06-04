package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"kiro2api/auth"
	"kiro2api/logger"
	"kiro2api/server"

	"github.com/joho/godotenv"
)

func main() {
	// 自动加载.env文件
	if err := godotenv.Load(); err != nil {
		logger.Info("未找到.env文件，使用环境变量")
	}

	// 重新初始化logger以使用.env文件中的配置
	logger.Reinitialize()

	// 检查子命令
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "get-token":
			runGetTokenCommand()
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
	refreshToken, err := auth.GetDeviceFlowToken(*authType)
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: %v\n", err)
		os.Exit(1)
	}

	// 构建配置
	config := []auth.AuthConfig{
		{
			AuthType:     *authType,
			RefreshToken: refreshToken,
			Disabled:     false,
		},
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error("序列化配置失败", logger.Err(err))
		fmt.Fprintf(os.Stderr, "❌ 错误: 无法序列化配置\n")
		os.Exit(1)
	}

	// 输出或保存到文件
	if *output != "" {
		// 保存到文件（权限: 0600）
		if err := os.WriteFile(*output, jsonData, 0600); err != nil {
			logger.Error("写入文件失败", logger.Err(err))
			fmt.Fprintf(os.Stderr, "❌ 错误: 无法写入文件 %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Printf("✓ Token已保存到: %s\n", *output)
		fmt.Println("\n接下来，你可以这样启动服务器:")
		fmt.Printf("  KIRO_AUTH_TOKEN=%s ./kiro2api\n", *output)
	} else {
		// 打印到标准输出
		fmt.Println("⚠️  警告: Token包含敏感信息，请妥善保管")
		fmt.Println("\n配置内容:")
		fmt.Println(string(jsonData))
		fmt.Println("\n你可以将上述内容保存到文件，例如:")
		fmt.Println("  ./kiro2api get-token --output auth_config.json")
	}
}

func printHelp() {
	fmt.Println("kiro2api - AWS CodeWhisperer API 代理服务器")
	fmt.Println("\n用法:")
	fmt.Println("  ./kiro2api [PORT]              启动服务器 (默认端口: 8080)")
	fmt.Println("  ./kiro2api get-token           获取AWS SSO token (交互式)")
	fmt.Println("  ./kiro2api get-token [OPTIONS] 获取token并保存配置")
	fmt.Println("\n子命令:")
	fmt.Println("  get-token                      启动AWS SSO设备授权流程")
	fmt.Println("                                 交互式显示登录链接和验证码")
	fmt.Println("\n选项:")
	fmt.Println("  -output string                 输出文件路径（默认：打印到终端）")
	fmt.Println("  -type string                   认证类型: Social 或 IdC (默认: Social)")
	fmt.Println("\n示例:")
	fmt.Println("  ./kiro2api                     启动默认服务器 (8080)")
	fmt.Println("  ./kiro2api 9000                启动服务器 (9000)")
	fmt.Println("  PORT=3000 ./kiro2api           启动服务器 (3000)")
	fmt.Println("  ./kiro2api get-token           开始认证流程")
	fmt.Println("  ./kiro2api get-token --output auth.json  保存到文件")
	fmt.Println("  ./kiro2api get-token --type IdC  使用IdC认证")
	fmt.Println("\n环境变量:")
	fmt.Println("  PORT                  服务端口 (默认: 8080)")
	fmt.Println("  KIRO_CLIENT_TOKEN     API认证密钥 (必需)")
	fmt.Println("  KIRO_AUTH_TOKEN       AWS认证配置 (必需)")
	fmt.Println("  LOG_LEVEL             日志级别 (debug/info/warn/error)")
	fmt.Println("  LOG_FORMAT            日志格式 (text/json)")
}

