package auth

import (
	"bytes"
	"fmt"
	"io"
	"kiro2api/logger"
	"kiro2api/utils"
	"net/http"
	"time"
)

const (
	oidcBaseURL = "https://oidc.us-east-1.amazonaws.com"
	startURL    = "https://codewhisperer.us-east-1.amazonaws.com"
)

// ClientRegistration 客户端注册响应
type ClientRegistration struct {
	ClientID                string `json:"clientId"`
	ClientSecret           string `json:"clientSecret"`
	ClientIDIssuedAt        int64  `json:"clientIdIssuedAt"`
	ClientSecretExpiresAt   int64  `json:"clientSecretExpiresAt"`
}

// DeviceAuthorizationResponse 设备授权响应
type DeviceAuthorizationResponse struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
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
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// GetDeviceFlowToken 执行设备授权流程获取token
func GetDeviceFlowToken(authType string) (string, error) {
	logger.Info("启动AWS SSO设备授权流程")

	// 步骤1: 注册客户端
	logger.Debug("步骤1: 注册客户端")
	clientReg, err := registerClient()
	if err != nil {
		return "", fmt.Errorf("客户端注册失败: %w", err)
	}
	logger.Debug("客户端注册成功",
		logger.String("clientId", clientReg.ClientID))

	// 步骤2: 启动设备授权
	logger.Debug("步骤2: 启动设备授权")
	deviceAuth, err := startDeviceAuthorization(clientReg.ClientID, clientReg.ClientSecret)
	if err != nil {
		return "", fmt.Errorf("启动设备授权失败: %w", err)
	}

	// 显示用户验证信息
	logger.Info("请打开浏览器访问以下链接进行认证:")
	fmt.Printf("\n📱 验证链接: %s\n", deviceAuth.VerificationURI)
	fmt.Printf("📝 验证码: %s\n\n", deviceAuth.UserCode)
	fmt.Println("正在等待您的授权... (按 Ctrl+C 取消)")

	// 步骤3: 轮询获取token
	logger.Debug("步骤3: 轮询获取token")
	refreshToken, err := pollForToken(clientReg.ClientID, clientReg.ClientSecret, deviceAuth.DeviceCode, deviceAuth.ExpiresIn, deviceAuth.Interval)
	if err != nil {
		return "", fmt.Errorf("获取token失败: %w", err)
	}

	fmt.Println("\n✓ 授权成功！")
	return refreshToken, nil
}

// registerClient 注册OAuth客户端
func registerClient() (*ClientRegistration, error) {
	url := oidcBaseURL + "/client/register"
	payload := []byte(`{"clientName":"kiro2api","clientType":"public"}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-cli/2.0 python/3.8.0")

	resp, err := utils.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("注册失败: 状态码 %d, 响应: %s", resp.StatusCode, string(body))
	}

	var clientReg ClientRegistration
	if err := utils.SafeUnmarshal(body, &clientReg); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &clientReg, nil
}

// startDeviceAuthorization 启动设备授权流程
func startDeviceAuthorization(clientID, clientSecret string) (*DeviceAuthorizationResponse, error) {
	url := oidcBaseURL + "/device_authorization"
	payload := []byte(fmt.Sprintf(
		`{"clientId":"%s","clientSecret":"%s","startUrl":"%s"}`,
		clientID, clientSecret, startURL))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-cli/2.0 python/3.8.0")

	resp, err := utils.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("启动设备授权失败: 状态码 %d, 响应: %s", resp.StatusCode, string(body))
	}

	var deviceAuth DeviceAuthorizationResponse
	if err := utils.SafeUnmarshal(body, &deviceAuth); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &deviceAuth, nil
}

// pollForToken 轮询等待用户授权并获取token
func pollForToken(clientID, clientSecret, deviceCode string, expiresIn, interval int) (string, error) {
	url := oidcBaseURL + "/token"
	startTime := time.Now()
	timeout := time.Duration(expiresIn) * time.Second
	pollInterval := time.Duration(interval) * time.Second

	// 确保轮询间隔至少1秒
	if pollInterval < time.Second {
		pollInterval = time.Second
	}

	for {
		// 检查是否超时
		if time.Since(startTime) > timeout {
			return "", fmt.Errorf("设备授权超时 (%d秒)", expiresIn)
		}

		// 构建请求
		payload := []byte(fmt.Sprintf(
			`{"clientId":"%s","clientSecret":"%s","grantType":"urn:ietf:params:oauth:grant-type:device_code","deviceCode":"%s"}`,
			clientID, clientSecret, deviceCode))

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
		if err != nil {
			logger.Debug("创建请求失败", logger.Err(err))
			time.Sleep(pollInterval)
			continue
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "aws-cli/2.0 python/3.8.0")

		resp, err := utils.SharedHTTPClient.Do(req)
		if err != nil {
			logger.Debug("HTTP请求失败", logger.Err(err))
			time.Sleep(pollInterval)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 检查授权状态
		if resp.StatusCode == http.StatusOK {
			var tokenResp TokenResponse
			if err := utils.SafeUnmarshal(body, &tokenResp); err != nil {
				logger.Debug("解析token响应失败", logger.Err(err))
				time.Sleep(pollInterval)
				continue
			}

			logger.Debug("成功获取token",
				logger.String("tokenType", tokenResp.TokenType),
				logger.Int("expiresIn", tokenResp.ExpiresIn))

			return tokenResp.RefreshToken, nil
		}

		// 检查授权待处理或其他错误
		var errResp ErrorResponse
		if err := utils.SafeUnmarshal(body, &errResp); err == nil {
			if errResp.Error == "authorization_pending" {
				logger.Debug("授权待处理，继续轮询")
			} else if errResp.Error == "slow_down" {
				logger.Debug("请求过于频繁，增加轮询间隔")
				pollInterval += 5 * time.Second
			} else if errResp.Error == "expired_token" {
				return "", fmt.Errorf("设备码已过期")
			} else {
				logger.Debug("授权错误",
					logger.String("error", errResp.Error),
					logger.String("description", errResp.ErrorDescription))
			}
		} else {
			logger.Debug("未知错误响应",
				logger.String("status", fmt.Sprintf("%d", resp.StatusCode)),
				logger.String("body", string(body)))
		}

		time.Sleep(pollInterval)
	}
}
