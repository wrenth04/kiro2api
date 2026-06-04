# 多平台發布指南

## 概述

此專案使用 GitHub Actions 自動為多個平台編譯和發布二進制版本。支持以下平台：

- **Linux**: amd64, arm64
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64, arm64

## 發布流程

### 自動發布（推薦）

1. **創建並推送 release tag**：
```bash
git tag v1.2.3
git push origin v1.2.3
```

2. **GitHub Actions 自動觸發**：
   - 自動為所有 6 個平台編譯二進制
   - 生成 SHA256 校驗和
   - 創建 GitHub Release 並上傳所有文件

3. **檢查發布結果**：
   - 前往 [Releases](https://github.com/YOUR_ORG/kiro2api/releases) 頁面
   - 驗證所有平台的二進制文件都已上傳

### 手動發布

在 GitHub Actions 頁面手動觸發工作流程：

1. 前往 **Actions** 標籤
2. 選擇 **Multi-Platform Release Build**
3. 點擊 **Run workflow**
4. 輸入 tag 名稱（例如 `v1.2.3`）

## 工作流程結構

### Job 1: build
- **目的**：為每個平台編譯二進制
- **矩陣策略**：6 個平台並行編譯
- **產出**：
  - 二進制文件（例如 `kiro2api-linux-amd64`）
  - SHA256 校驗和文件

### Job 2: test-artifacts
- **目的**：驗證編譯成功
- **運行時機**：build 完成後
- **驗證內容**：校驗和有效性

### Job 3: create-release
- **目的**：創建 GitHub Release
- **運行時機**：tag push 時自動觸發
- **產出**：
  - GitHub Release 頁面
  - 所有二進制文件下載鏈接
  - 校驗和文檔

## 編譯設置

### 構建參數

```bash
go build -o <binary-name> \
  -ldflags="-s -w -X main.Version=<version>" \
  main.go
```

- `-s -w`：移除調試符號，減小文件大小
- `-X main.Version=<version>`：注入版本信息（需在 main.go 中定義）
- `CGO_ENABLED=0`：禁用 CGO，提高兼容性

### 支持的 Go 版本

- Go 1.24.0（當前版本）

## 下載和驗證

### 下載二進制

```bash
# Linux amd64
wget https://github.com/YOUR_ORG/kiro2api/releases/download/v1.2.3/kiro2api-linux-amd64

# macOS arm64 (Apple Silicon)
wget https://github.com/YOUR_ORG/kiro2api/releases/download/v1.2.3/kiro2api-darwin-arm64

# Windows amd64
wget https://github.com/YOUR_ORG/kiro2api/releases/download/v1.2.3/kiro2api-windows-amd64.exe
```

### 驗證校驗和

```bash
# 下載校驗和文件
wget https://github.com/YOUR_ORG/kiro2api/releases/download/v1.2.3/kiro2api-linux-amd64.sha256

# 驗證
sha256sum -c kiro2api-linux-amd64.sha256
```

### 使用二進制

```bash
# Linux/macOS
chmod +x kiro2api-linux-amd64
./kiro2api-linux-amd64

# Windows
kiro2api-windows-amd64.exe
```

## 故障排除

### 編譯失敗

1. 檢查 Go 版本：`go version`
2. 清理依賴：`go mod tidy && go mod download`
3. 查看 GitHub Actions 日誌獲取詳細信息

### Release 未創建

- 確保 tag 以 `v` 開頭（例如 `v1.2.3`）
- 檢查權限設置（需要 `contents: write`）
- 查看 workflow 的 "Create GitHub Release" step 日誌

### 校驗和驗證失敗

- 確保下載完整（文件大小相符）
- 使用相同平台的校驗和工具
- 文件名必須完全匹配

## 性能提示

- **並行編譯**：所有 6 個平台同時編譯，總耗時約 5-10 分鐘
- **GitHub Actions 緩存**：Go 模塊自動緩存，加速後續編譯
- **工件保留**：臨時工件保留 5 天后自動刪除

## 擴展和自定義

### 添加新平台

編輯 `multi-platform-release.yml` 中的 `strategy.matrix.include` 部分：

```yaml
- os: linux
  arch: arm
  runner: ubuntu-latest
  output_name: kiro2api-linux-arm
```

### 修改版本注入

在 `main.go` 中定義版本變量：

```go
package main

var Version string = "dev"

func main() {
    logger.Info("Version", logger.String("version", Version))
}
```

### 自定義構建選項

在 workflow 的 build step 中修改 `-ldflags`：

```yaml
-ldflags="-s -w -X main.Version=${{ steps.version.outputs.version }} -X main.BuildTime=$(date)"
```

## 相關文件

- 工作流文件：`.github/workflows/multi-platform-release.yml`
- Docker 發布：`.github/workflows/release-build.yml`（Docker 鏡像）
- 主程序：`main.go`
- 模塊定義：`go.mod`
