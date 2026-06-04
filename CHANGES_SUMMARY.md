# 改动总结：Token 管理策略优化

## 📝 改动概览

本次更新包含两个核心改进：

### 1️⃣ Token 选择策略调整：顺序耗尽策略
### 2️⃣ get-token 命令增强：自动追加到现有配置

---

## 🔄 改动 1：顺序耗尽策略（Sequential Exhaustion Strategy）

### 📍 文件：`auth/token_manager.go`

#### 变更内容

**旧策略（轮转策略 Round-robin）：**
```
请求1 → token_0 ✓ → 下次用 token_1
请求2 → token_1 ✓ → 下次用 token_2
请求3 → token_2 ✓ → 下次用 token_0（循环）
```
问题：token 池中的 token 轮流被使用，无法充分利用单个 token 的额度。

**新策略（顺序耗尽策略 Sequential Exhaustion）：**
```
请求1-5 → token_0（可用5次）✓ → 用完
请求6-10 → token_1（可用5次）✓ → 用完
请求11-15 → token_2（可用5次）✓ → 用完
请求16 → token_0（如已恢复）✓
```
优点：充分利用每个 token 的配额，减少不必要的切换。

#### 核心改动

1. **移除 `exhausted` 字段**
   ```go
   // 旧：
   exhausted map[string]bool
   
   // 新：直接通过 selectBestTokenUnlocked() 逻辑实现
   ```

2. **修改 `selectBestTokenUnlocked()` 方法**
   ```go
   // 新逻辑：
   // 1. 先检查当前 token 是否仍可用
   if currentToken.IsUsable() {
     return currentToken  // 继续使用当前 token
   }
   
   // 2. 当前 token 已耗尽，寻找下一个可用的
   for i := 0; i < len(configOrder); i++ {
     nextIndex = (currentIndex + 1) % len(configOrder)
     if nextToken.IsUsable() {
       currentIndex = nextIndex  // 更新为新的当前 token
       return nextToken
     }
   }
   ```

3. **更新初始化日志**
   ```go
   logger.Info("TokenManager初始化（顺序耗尽策略）", ...)
   ```

#### 测试验证

```bash
go test ./auth -v -run TestTokenManager_SequentialSelection
# 结果：✅ PASS
```

测试输出显示：
```
✓ 顺序耗尽策略：继续使用当前token
✓ 顺序耗尽策略：切换到下一个token
✓ Token选择分布: map[access_0:5 access_1:5 access_2:5]
✅ 顺序选择策略验证通过：粘性策略正确工作
```

---

## 🚀 改动 2：get-token 命令增强

### 📍 文件：`main.go`

#### 变更内容

**旧行为：**
- 总是创建新文件或覆盖现有文件
- 需要手动编辑 JSON 来追加 token

**新行为：**
- 自动检测 `KIRO_AUTH_TOKEN` 环境变量
- 如果指向现有文件，自动追加新 token 到列表
- 保留现有的 token 配置

#### 核心改动

1. **新增环境变量检测**
   ```go
   // 如果未指定 --output，检查环境变量
   envPath := os.Getenv("KIRO_AUTH_TOKEN")
   if envPath != "" {
     if fileInfo, err := os.Stat(envPath); err == nil && !fileInfo.IsDir() {
       outputPath = envPath  // 自动使用环境变量指向的文件
     }
   }
   ```

2. **新增文件读取和追加逻辑**
   ```go
   // 如果输出路径指向现有文件，读取并追加
   if outputPath != "" && fileExists {
     existingConfigs := readExistingConfig(outputPath)
     config = append(existingConfigs, newTokenConfig)
   }
   ```

3. **新增辅助函数**
   ```go
   // 检查是否是全新配置（只有新 token）
   func isNewConfig(config []AuthConfig, newToken AuthConfig) bool {
     return len(config) == 1 && 
            config[0].RefreshToken == newToken.RefreshToken
   }
   ```

#### 使用示例

**初次获取 token：**
```bash
./kiro2api get-token --output config.json
# 创建新文件，包含第一个 token
```

**追加第二个 token（核心新功能）：**
```bash
export KIRO_AUTH_TOKEN=config.json
./kiro2api get-token
# 自动追加到 config.json，现在包含 2 个 token
```

**追加第三个 token（不同类型）：**
```bash
./kiro2api get-token --type IdC
# 自动追加 IdC token，现在包含 2 个 Social + 1 个 IdC
```

---

## 🔧 Bug 修复

### 📍 文件：`auth/usage_checker.go`

**问题：** Token 长度检查不安全，导致切片越界

```go
// 旧代码（有 bug）：
logger.String("token_preview", token.AccessToken[:20]+"...")
// 当 token 长度 < 20 时会 panic

// 新代码（安全）：
tokenPreview := token.AccessToken
if len(tokenPreview) > 20 {
  tokenPreview = tokenPreview[:20] + "..."
}
logger.String("token_preview", tokenPreview)
```

**测试验证：**
```bash
go test ./auth -v
# 结果：✅ PASS（无 panic）
```

---

## 📊 改动影响分析

### 性能影响

| 指标 | 旧策略 | 新策略 | 改进 |
|------|-------|-------|------|
| Token 切换频率 | 每请求轮转 | 仅在耗尽时 | ✅ 减少 |
| 缓存查询次数 | 每次 O(n) | 平均 O(1) | ✅ 显著 |
| 内存占用 | 无 `exhausted` 字典 | 无 | ✅ 相同 |
| 并发安全性 | 单一 `mutex` | 单一 `mutex` | ✅ 相同 |

### 功能影响

| 功能 | 旧实现 | 新实现 | 兼容性 |
|------|--------|--------|--------|
| Token 池轮转 | Round-robin | 顺序耗尽 | ✅ 增强 |
| 配置文件追加 | 不支持 | 自动检测 | ✅ 新增 |
| 环境变量检测 | 无 | 自动检测 | ✅ 新增 |
| 向后兼容性 | - | - | ✅ 完全兼容 |

---

## 🧪 测试覆盖

所有现有测试通过：

```bash
go test ./... -v

# 关键测试结果：
✅ auth/token_manager_test.go
   - TestTokenManager_SequentialSelection (顺序耗尽策略验证)
   - TestTokenManager_RaceCondition (并发安全性)
   - 所有其他 auth 测试

✅ auth/usage_checker_test.go
   - 修复后的 token 预览逻辑

✅ 所有其他模块测试
   - server, parser, converter, utils, logger 等

总计：所有测试 PASS ✅
```

---

## 📋 文件变更清单

```
修改的文件：
├── auth/token_manager.go          ✏️  修改选择策略、移除 exhausted
├── auth/usage_checker.go          🐛 修复 token 预览越界 bug
├── main.go                        ✨ 增强 get-token 命令
└── GET_TOKEN_USAGE.md             📖 新增使用文档

编译验证：
✅ go build -o kiro2api main.go    (无编译错误)
✅ go test ./...                   (所有测试通过)
```

---

## 🚀 升级指南

### 现有用户

**无需改动！** 这是向后兼容的更新。

已有的配置文件和环境变量无需修改，系统会自动使用新的顺序耗尽策略。

### 新用户

**推荐工作流：**

1. 获取第一个 token
   ```bash
   ./kiro2api get-token --output config.json
   ```

2. 根据需要追加更多 token
   ```bash
   export KIRO_AUTH_TOKEN=config.json
   ./kiro2api get-token               # 追加 Social token
   ./kiro2api get-token --type IdC   # 追加 IdC token
   ```

3. 启动服务
   ```bash
   export KIRO_AUTH_TOKEN=config.json
   export KIRO_CLIENT_TOKEN=$(openssl rand -base64 32)
   ./kiro2api
   ```

---

## 💡 优化后的行为示例

### 场景：3 个 token，每个可用 5 次

**旧策略输出：**
```
请求1  → token_0.available=5 (选中) → token_0.available=4
请求2  → token_1.available=5 (选中) → token_1.available=4
请求3  → token_2.available=5 (选中) → token_2.available=4
请求4  → token_0.available=4 (选中) → token_0.available=3
请求5  → token_1.available=4 (选中) → token_1.available=3
```
结果：15 个请求分散到 3 个 token，轮流使用

**新策略输出：**
```
请求1  → token_0.available=5 (选中) → token_0.available=4
请求2  → token_0.available=4 (选中) → token_0.available=3
请求3  → token_0.available=3 (选中) → token_0.available=2
请求4  → token_0.available=2 (选中) → token_0.available=1
请求5  → token_0.available=1 (选中) → token_0.available=0
请求6  → token_0.available=0 (不可用) → 切换到 token_1
请求7  → token_1.available=5 (选中) → token_1.available=4
...
请求15 → token_2.available=1 (选中) → token_2.available=0
```
结果：15 个请求依次耗尽 3 个 token，充分利用配额

---

## ✅ 变更验收清单

- ✅ 代码编译无错误
- ✅ 所有现有测试通过
- ✅ 新增功能测试通过
- ✅ 并发安全性验证
- ✅ 向后兼容性确认
- ✅ 文档完整（GET_TOKEN_USAGE.md）
- ✅ 日志输出清晰
- ✅ 错误处理完善

---

## 📚 相关文档

- `GET_TOKEN_USAGE.md` - get-token 命令完整使用指南
- `auth/token_manager.go` - Token 管理器核心实现
- `auth/config.go` - 配置加载和验证
- `main.go` - 命令行工具入口

