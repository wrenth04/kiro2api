# kiro2api 設定指南

## 完整工作流程

### 步驟 1: 獲取 AWS SSO Token

```bash
./kiro2api get-token --output auth.json
```

這會啟動交互式 AWS SSO 設備授權流程：
- 顯示驗證鏈接
- 顯示驗證碼
- 輪詢等待用戶授權
- 保存 `refreshToken` 到 `auth.json`

**輸出示例：**
```json
[
  {
    "auth": "Social",
    "refreshToken": "aorAAAAAG7xxxxx...",
    "disabled": false
  }
]
```

### 步驟 2: 自動設定環境

```bash
./kiro2api setup --auth auth.json
```

這會自動：
- ✅ 讀取 `auth.json` 配置
- ✅ 生成隨機客戶端 token
- ✅ 創建 `.env.local` 配置文件
- ✅ 顯示啟動命令

**輸出示例：**
```
============================================================
  ✓ 配置完成！
============================================================

📝 配置文件: .env.local
🔐 客戶端token: _4HrexlU1cD7KtWLcathWahGKWovOIpK
🌐 服務端口: 8080
📂 認證配置: /full/path/to/auth.json

------------------------------------------------------------
  啟動服務器:
------------------------------------------------------------

  export $(cat .env.local | xargs) && ./kiro2api

或者：

  PORT=8080 KIRO_CLIENT_TOKEN=_4HrexlU1cD7KtWLcathWahGKWovOIpK KIRO_AUTH_TOKEN=/full/path/to/auth.json ./kiro2api
```

### 步驟 3: 啟動服務器

**方法 A: 使用環境變數直接啟動（推薦）**
```bash
export KIRO_AUTH_TOKEN=/path/to/auth.json
export KIRO_CLIENT_TOKEN=your_client_token
export PORT=8080
./kiro2api
```

**方法 B: 一條命令啟動**
```bash
PORT=8080 KIRO_CLIENT_TOKEN=your_token KIRO_AUTH_TOKEN=/path/to/auth.json ./kiro2api
```

**方法 C: 使用 .env.local（需要正確的 shell）**
```bash
# 在 bash 中
set -a
source .env.local
set +a
./kiro2api
```

或使用 `export` 命令：
```bash
export $(cat .env.local | grep -v '^#' | xargs)
./kiro2api
```

## setup 命令選項

### 基本用法
```bash
./kiro2api setup --auth auth.json
```

### 自定義設定
```bash
./kiro2api setup \
  --auth auth.json \
  --port 9000 \
  --client-token your-secure-token \
  --env .env.production
```

### 選項說明

| 選項 | 必需 | 默認值 | 說明 |
|------|------|--------|------|
| `--auth` | ✅ | - | 認證配置文件路徑（由 `get-token` 生成） |
| `--client-token` | ❌ | 自動生成 | 用於保護 API 的客戶端 token |
| `--port` | ❌ | 8080 | HTTP 服務端口 |
| `--env` | ❌ | .env.local | 輸出的環境配置文件路徑 |

## 生成的 .env.local 文件

```bash
# kiro2api 配置文件（自動生成）
# 生成時間: 2026-06-04 05:34:38

# 服務端口
PORT=8080

# 客戶端認證token（用於保護API）
KIRO_CLIENT_TOKEN=_4HrexlU1cD7KtWLcathWahGKWovOIpK

# 認證配置文件路徑
KIRO_AUTH_TOKEN=/path/to/auth.json

# 日誌配置
LOG_LEVEL=info
LOG_FORMAT=json

# 工具描述長度限制（字符數）
TOOL_DESCRIPTION_MAX_LENGTH=500
```

## 環境變數說明

| 變數 | 必需 | 說明 |
|------|------|------|
| `KIRO_AUTH_TOKEN` | ✅ | AWS 認證配置，支持文件路徑或 JSON 字符串 |
| `KIRO_CLIENT_TOKEN` | ✅ | API 認證密鑰（用於保護服務器） |
| `PORT` | ❌ | 服務端口，默認 8080 |
| `LOG_LEVEL` | ❌ | 日誌級別：debug/info/warn/error，默認 info |
| `LOG_FORMAT` | ❌ | 日誌格式：text/json，默認 json |
| `TOOL_DESCRIPTION_MAX_LENGTH` | ❌ | 工具描述長度限制，默認 500 |

## 認證類型

### Social (AWS Builder ID) - 默認
用於個人開發者或企業員工的 AWS Builder ID 登入。

```bash
./kiro2api get-token --type Social --output auth.json
```

### IdC (IAM Identity Center)
用於企業環境的 IAM Identity Center 登入。

```bash
./kiro2api get-token --type IdC --output auth.json
```

## 完整示例

### 場景 1: 本地開發

```bash
# 1. 獲取 token
./kiro2api get-token --type Social --output auth.json

# 2. 設定環境
./kiro2api setup --auth auth.json

# 3. 查看配置
cat .env.local

# 4. 啟動服務
export $(cat .env.local | grep -v '^#' | xargs)
./kiro2api
```

### 場景 2: 企業 IdC 環境

```bash
# 1. 獲取 token（使用 IdC）
./kiro2api get-token --type IdC --output auth_idc.json

# 2. 設定環境（自定義端口和 token）
./kiro2api setup \
  --auth auth_idc.json \
  --port 9000 \
  --client-token "my-secure-enterprise-token" \
  --env .env.production

# 3. 啟動服務
export $(cat .env.production | grep -v '^#' | xargs)
./kiro2api
```

### 場景 3: 直接使用環境變數

```bash
PORT=8080 \
KIRO_CLIENT_TOKEN=secure-token \
KIRO_AUTH_TOKEN=/path/to/auth.json \
./kiro2api
```

## 常見問題

### Q1: 如何驗證配置是否正確？
```bash
# 查看 .env.local 的內容
cat .env.local

# 驗證環境變數是否設置
echo $KIRO_AUTH_TOKEN
echo $KIRO_CLIENT_TOKEN
echo $PORT
```

### Q2: 如何更改端口？
```bash
./kiro2api setup --auth auth.json --port 9000
```

### Q3: 如何更改客戶端 token？
```bash
./kiro2api setup --auth auth.json --client-token "new-secure-token"
```

### Q4: 使用 .env.local 時出現環境變數未設置？
確保使用以下命令載入環境變數：
```bash
# 方法 1: 直接 export
export $(cat .env.local | grep -v '^#' | xargs)

# 方法 2: 使用 set -a
set -a && source .env.local && set +a

# 方法 3: 使用環境變數直接啟動（推薦）
export KIRO_AUTH_TOKEN=/path/to/auth.json
export KIRO_CLIENT_TOKEN=token
./kiro2api
```

### Q5: 如何在多個環境中使用？
```bash
# 開發環境
./kiro2api setup --auth auth_dev.json --port 8080 --env .env.dev

# 測試環境
./kiro2api setup --auth auth_test.json --port 8081 --env .env.test

# 生產環境
./kiro2api setup --auth auth_prod.json --port 80 --env .env.prod
```

## 安全建議

⚠️ **重要提醒：**

1. **保護 auth.json**
   - 包含敏感的 refresh token
   - 添加到 `.gitignore`
   - 設置正確的文件權限：`chmod 600 auth.json`

2. **保護 .env.local**
   - 包含客戶端 token
   - 添加到 `.gitignore`
   - 設置正確的文件權限：`chmod 600 .env.local`

3. **客戶端 Token**
   - 用於保護 API 接口
   - 應該是強密碼（32+ 字符）
   - 定期輪換

4. **環境變數**
   - 避免在命令行歷史中暴露敏感信息
   - 使用文件或環境管理工具

## 故障排除

### 錯誤：未找到 KIRO_AUTH_TOKEN

確保：
1. `auth.json` 文件存在
2. 文件路徑正確
3. 環境變數已設置

```bash
# 驗證文件
ls -la /path/to/auth.json

# 驗證環境變數
echo $KIRO_AUTH_TOKEN
```

### 錯誤：Token 無效或已過期

重新獲取 token：
```bash
./kiro2api get-token --output auth.json
./kiro2api setup --auth auth.json
```

### 服務器無法啟動

檢查日誌：
```bash
# 使用 debug 日誌級別
LOG_LEVEL=debug KIRO_CLIENT_TOKEN=test KIRO_AUTH_TOKEN=/path/to/auth.json ./kiro2api
```

## 幫助命令

```bash
# 查看所有選項
./kiro2api --help

# 查看 get-token 幫助
./kiro2api get-token --help

# 查看 setup 幫助
./kiro2api setup --help
```
