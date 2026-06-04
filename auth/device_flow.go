package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	kconfig "kiro2api/config"
	"kiro2api/logger"
	"kiro2api/utils"
	"net/http"
	"strings"
	"time"
)

const (
	oidcBaseURL = "https://oidc.us-east-1.amazonaws.com"

	// Auth types
	AuthTypeBuilderID = "Social"
	AuthTypeIdC       = "IdC"

	// Builder ID configuration
	builderIDStartURL = "https://view.awsapps.com/start"
)

// ClientRegistration 客户端注册响应
type ClientRegistration struct {
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret"`
	ClientIDIssuedAt      int64    `json:"clientIdIssuedAt"`
	ClientSecretExpiresAt int64    `json:"clientSecretExpiresAt"`
	Scopes                []string `json:"scopes,omitempty"`
	GrantTypes            []string `json:"grantTypes,omitempty"`
}

// DeviceAuthorizationResponse 设备授权响应
type DeviceAuthorizationResponse struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete,omitempty"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// TokenResponse OAuth token响应
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}

// ErrorResponse OAuth错误响应
type ErrorResponse struct {
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Error 实现error接口
func (e *ErrorResponse) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorDescription)
	}
	return e.ErrorCode
}

// DeviceFlowConfig 设备授权流配置
type DeviceFlowConfig struct {
	AuthType  string
	StartURL  string
	IssuerURL string
	Region    string
}

// GetDeviceFlowTokenResult 设备流程的返回结果
type GetDeviceFlowTokenResult struct {
	AccessToken  string
	RefreshToken string
}

// GetDeviceFlowToken 执行AWS SSO设备授权流程获取token
func GetDeviceFlowToken(authType string) (GetDeviceFlowTokenResult, error) {
	logger.Info("启动AWS SSO设备授权流程",
		logger.String("authType", authType))

	// 构建配置
	config := buildDeviceFlowConfig(authType)

	// 步骤1: 注册OIDC客户端
	logger.Debug("步骤1: 注册OIDC客户端")
	clientReg, err := registerClient(config)
	if err != nil {
		return GetDeviceFlowTokenResult{}, fmt.Errorf("客户端注册失败: %w", err)
	}
	logger.Debug("客户端注册成功",
		logger.String("clientId", clientReg.ClientID),
		logger.Int("scopeCount", len(clientReg.Scopes)))

	// 步骤2: 启动设备授权
	logger.Debug("步骤2: 启动设备授权")
	deviceAuth, err := startDeviceAuthorization(clientReg, config)
	if err != nil {
		return GetDeviceFlowTokenResult{}, fmt.Errorf("启动设备授权失败: %w", err)
	}

	// 显示用户验证信息
	displayVerificationInfo(deviceAuth)

	// 步骤3: 轮询获取token
	logger.Debug("步骤3: 轮询获取token")
	tokenResp, err := pollForToken(clientReg, deviceAuth)
	if err != nil {
		return GetDeviceFlowTokenResult{}, fmt.Errorf("获取token失败: %w", err)
	}

	fmt.Println("\n✓ 授权成功！")
	return GetDeviceFlowTokenResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}, nil
}

// buildDeviceFlowConfig 根据认证类型构建配置
func buildDeviceFlowConfig(authType string) *DeviceFlowConfig {
	config := &DeviceFlowConfig{
		AuthType: authType,
		Region:   "us-east-1",
	}

	switch authType {
	case AuthTypeBuilderID:
		config.StartURL = builderIDStartURL
	case AuthTypeIdC:
		config.StartURL = builderIDStartURL
		config.IssuerURL = "https://identitycenter.amazonaws.com/ssoins-722374e8c3c8e6c6"
	default:
		logger.Warn("Unknown auth type, defaulting to Builder ID",
			logger.String("authType", authType))
		config.AuthType = AuthTypeBuilderID
		config.StartURL = builderIDStartURL
	}

	return config
}

// registerClient 注册OIDC客户端
func registerClient(config *DeviceFlowConfig) (*ClientRegistration, error) {
	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/client/register", config.Region)

	// 构建注册负载，包含CodeWhisperer scopes
	payload := map[string]interface{}{
		"clientName": "kiro2api",
		"clientType": "public",
		"scopes": []string{
			"codewhisperer:completions",
			"codewhisperer:analysis",
			"codewhisperer:conversations",
		},
		"grantTypes": []string{
			"urn:ietf:params:oauth:grant-type:device_code",
			"refresh_token",
		},
	}

	// 为IdC添加issuerUrl
	if config.AuthType == AuthTypeIdC && config.IssuerURL != "" {
		payload["issuerUrl"] = config.IssuerURL
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", kconfig.UserAgentOIDC)

	resp, err := utils.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed: status %d, body: %s",
			resp.StatusCode, string(body))
	}

	var clientReg ClientRegistration
	if err := json.Unmarshal(body, &clientReg); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &clientReg, nil
}

// startDeviceAuthorization 启动设备授权流程
func startDeviceAuthorization(clientReg *ClientRegistration, config *DeviceFlowConfig) (*DeviceAuthorizationResponse, error) {
	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/device_authorization", config.Region)

	payload := map[string]string{
		"clientId":     clientReg.ClientID,
		"clientSecret": clientReg.ClientSecret,
		"startUrl":     config.StartURL,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", kconfig.UserAgentOIDC)

	resp, err := utils.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed: status %d, body: %s",
			resp.StatusCode, string(body))
	}

	var deviceAuth DeviceAuthorizationResponse
	if err := json.Unmarshal(body, &deviceAuth); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Debug("Device authorization started",
		logger.String("userCode", deviceAuth.UserCode),
		logger.Int("expiresIn", deviceAuth.ExpiresIn))

	return &deviceAuth, nil
}

// displayVerificationInfo 显示用户友好的验证信息
func displayVerificationInfo(deviceAuth *DeviceAuthorizationResponse) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  AWS SSO Device Authorization")
	fmt.Println(strings.Repeat("=", 60))

	if deviceAuth.VerificationURIComplete != "" {
		fmt.Printf("\n📱 Quick Link (includes code):\n   %s\n",
			deviceAuth.VerificationURIComplete)
	} else {
		fmt.Printf("\n📱 Verification URL:\n   %s\n", deviceAuth.VerificationURI)
		fmt.Printf("\n📝 Verification Code:\n   %s\n", deviceAuth.UserCode)
	}

	fmt.Printf("\n⏱  Expires in: %d seconds\n", deviceAuth.ExpiresIn)
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("  Waiting for authorization... (Press Ctrl+C to cancel)")
	fmt.Println(strings.Repeat("=", 60) + "\n")
}

// pollForToken 轮询等待用户授权并获取token
func pollForToken(clientReg *ClientRegistration, deviceAuth *DeviceAuthorizationResponse) (TokenResponse, error) {
	url := fmt.Sprintf("https://oidc.us-east-1.amazonaws.com/token")
	startTime := time.Now()
	timeout := time.Duration(deviceAuth.ExpiresIn) * time.Second
	pollInterval := time.Duration(deviceAuth.Interval) * time.Second

	// 最小轮询间隔为1秒
	if pollInterval < time.Second {
		pollInterval = time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否超时
			if time.Since(startTime) > timeout {
				return TokenResponse{}, fmt.Errorf("device authorization expired after %d seconds", deviceAuth.ExpiresIn)
			}

			// 尝试获取token
			token, err := attemptTokenFetch(url, clientReg, deviceAuth.DeviceCode)
			if err == nil {
				return token, nil
			}

			// 处理特定错误
			if errResp, ok := err.(*ErrorResponse); ok {
				switch errResp.ErrorCode {
				case "authorization_pending":
					// 继续轮询
					continue
				case "slow_down":
					// 增加轮询间隔
					pollInterval += 5 * time.Second
					ticker.Reset(pollInterval)
					logger.Debug("Slowing down polling",
						logger.String("newInterval", pollInterval.String()))
				case "expired_token":
					return TokenResponse{}, fmt.Errorf("device code expired")
				case "access_denied":
					return TokenResponse{}, fmt.Errorf("user denied authorization")
				default:
					logger.Warn("Polling error",
						logger.String("error_code", errResp.ErrorCode),
						logger.String("error_description", errResp.ErrorDescription))
				}
			}
		}
	}
}

// attemptTokenFetch 尝试一次获取token
func attemptTokenFetch(url string, clientReg *ClientRegistration, deviceCode string) (TokenResponse, error) {
	payload := map[string]string{
		"clientId":     clientReg.ClientID,
		"clientSecret": clientReg.ClientSecret,
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
		"deviceCode":   deviceCode,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return TokenResponse{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return TokenResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", kconfig.UserAgentOIDC)

	resp, err := utils.SharedHTTPClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, err
	}

	// 成功
	if resp.StatusCode == http.StatusOK {
		var tokenResp TokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return TokenResponse{}, err
		}

		logger.Info("Token acquired successfully",
			logger.String("tokenType", tokenResp.TokenType),
			logger.Int("expiresIn", tokenResp.ExpiresIn))

		return tokenResp, nil
	}

	// 错误响应
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return TokenResponse{}, fmt.Errorf("unexpected response: status %d", resp.StatusCode)
	}

	return TokenResponse{}, &errResp
}
