package config

import (
	"os"
	"strconv"
)

// ModelMap 模型映射表
var ModelMap = map[string]string{
	"claude-sonnet-4-5":          "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929": "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":   "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
	"claude-3-5-haiku-20241022":  "auto",
	"claude-haiku-4-5-20251001":  "auto",
}

// RefreshTokenURL 刷新token的URL (social方式)
// 使用与device flow相同的OIDC端点
const RefreshTokenURL = "https://oidc.us-east-1.amazonaws.com/token"

// IdcRefreshTokenURL IdC认证方式的刷新token URL
const IdcRefreshTokenURL = "https://oidc.us-east-1.amazonaws.com/token"

// CodeWhispererURL CodeWhisperer API的URL
const CodeWhispererURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"

// Kiro IDE 版本與 header 常數
const (
	KiroIDEVersion  = "1.0.0"
	KiroIDECommit   = "local"
	KiroIDEFullTag  = "KiroIDE-" + KiroIDEVersion + "-" + KiroIDECommit

	// AWS SDK 版本（codewhispererstreaming）
	AwsSDKStreamingVersion = "1.0.18"
	// AWS SDK 版本（codewhispererruntime / usage）
	AwsSDKRuntimeVersion = "1.0.0"
	// AWS SDK 版本（sso-oidc / refresh）
	AwsSDKOIDCVersion = "3.738.0"

	// OS / runtime 資訊（固定模擬 Kiro IDE 環境）
	KiroOSInfo      = "linux#5.10.0"
	KiroNodeVersion = "20.16.0"

	// x-amz-user-agent（用於 streaming API）
	XAmzUserAgentStreaming = "aws-sdk-js/" + AwsSDKStreamingVersion + " " + KiroIDEFullTag

	// user-agent（用於 streaming API）
	UserAgentStreaming = "aws-sdk-js/" + AwsSDKStreamingVersion + " ua/2.1 os/" + KiroOSInfo + " lang/js md/nodejs#" + KiroNodeVersion + " api/codewhispererstreaming#" + AwsSDKStreamingVersion + " m/E " + KiroIDEFullTag

	// x-amz-user-agent（用於 usage / runtime API）
	XAmzUserAgentRuntime = "aws-sdk-js/" + AwsSDKRuntimeVersion + " " + KiroIDEFullTag

	// user-agent（用於 usage / runtime API）
	UserAgentRuntime = "aws-sdk-js/" + AwsSDKRuntimeVersion + " ua/2.1 os/" + KiroOSInfo + " lang/js md/nodejs#" + KiroNodeVersion + " api/codewhispererruntime#" + AwsSDKRuntimeVersion + " m/E " + KiroIDEFullTag

	// x-amz-user-agent（用於 OIDC refresh）
	XAmzUserAgentOIDC = "aws-sdk-js/" + AwsSDKOIDCVersion + " ua/2.1 os/other lang/js md/browser#unknown_unknown api/sso-oidc#" + AwsSDKOIDCVersion + " m/E " + KiroIDEFullTag

	// User-Agent（用於 OIDC Social refresh HTTP header）
	UserAgentOIDC = "aws-sdk-js/" + AwsSDKOIDCVersion + " ua/2.1 os/other lang/js md/browser#unknown_unknown api/sso-oidc#" + AwsSDKOIDCVersion + " m/E " + KiroIDEFullTag
)

// MaxToolDescriptionLength 工具描述的最大长度（字符数）
// 可通过环境变量 MAX_TOOL_DESCRIPTION_LENGTH 配置，默认 10000
var MaxToolDescriptionLength = getEnvIntWithDefault("MAX_TOOL_DESCRIPTION_LENGTH", 10000)

// getEnvIntWithDefault 获取整数类型环境变量（带默认值）
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
