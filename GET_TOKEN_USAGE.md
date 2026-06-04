# get-token 命令使用指南

## 📋 改进内容

`get-token` 命令现在支持**自动追加到现有配置文件**，无需手动编辑 JSON。

### 核心特性

1. **自动检测环境变量** - 检查 `KIRO_AUTH_TOKEN` 是否指向现有文件
2. **自动追加 token** - 新 token 自动添加到现有配置列表
3. **保持现有配置** - 不会覆盖已有的 token
4. **灵活指定输出** - 支持 `--output` 参数明确指定输出路径

---

## 🚀 使用场景

### 场景 1：初次获取 token（打印到标准输出）

```bash
./kiro2api get-token
```

**输出：**
```
⚠️  警告: Token包含敏感信息，请妥善保管

配置内容:
[
  {
    "auth": "Social",
    "accessToken": "...",
    "refreshToken": "aws_refresh_token_xxx",
    "disabled": false
  }
]

你可以将上述内容保存到文件，例如:
  ./kiro2api get-token --output auth_config.json

或者自动追加到环境变量指定的文件:
  KIRO_AUTH_TOKEN=/path/to/config.json ./kiro2api get-token
```

---

### 场景 2：指定输出文件（新建配置）

```bash
./kiro2api get-token --output auth_config.json
```

**输出：**
```
✓ Token已保存到: /path/to/auth_config.json

接下来，你可以这样启动服务器:
  KIRO_AUTH_TOKEN=/path/to/auth_config.json ./kiro2api
```

**生成的文件：**
```json
[
  {
    "auth": "Social",
    "accessToken": "...",
    "refreshToken": "aws_refresh_token_user1",
    "disabled": false
  }
]
```

---

### 场景 3：追加新 token 到现有配置（核心新功能）

**初始状态：** 已有 `auth_config.json` 包含 2 个 token

```json
[
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user1",
    "disabled": false
  },
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user2",
    "disabled": false
  }
]
```

**命令：** 不指定输出路径，让系统自动检测环境变量

```bash
export KIRO_AUTH_TOKEN=/etc/kiro2api/auth_config.json
./kiro2api get-token
```

**过程日志：**
```
📂 检测到环境变量 KIRO_AUTH_TOKEN 指向文件: /etc/kiro2api/auth_config.json
✓ 读取现有配置 (2个token)
📝 将新token追加到配置列表
✓ 配置已保存到: /etc/kiro2api/auth_config.json
✓ 已将新token追加到配置列表 (现在共3个token)

接下来，你可以这样启动服务器:
  KIRO_AUTH_TOKEN=/etc/kiro2api/auth_config.json ./kiro2api
```

**更新后的文件：**
```json
[
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user1",
    "disabled": false
  },
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user2",
    "disabled": false
  },
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user3",
    "disabled": false
  }
]
```

---

### 场景 4：显式指定输出路径（覆盖模式）

```bash
./kiro2api get-token --output auth_config.json
```

**行为：**
- 如果 `auth_config.json` 已存在：**追加**新 token
- 如果 `auth_config.json` 不存在：**创建**新文件

**输出：**
```
✓ 读取现有配置 (2个token)
📝 将新token追加到配置列表
✓ 配置已保存到: auth_config.json
✓ 已将新token追加到配置列表 (现在共3个token)
```

---

### 场景 5：获取不同类型的 token（IdC）

```bash
export KIRO_AUTH_TOKEN=/etc/kiro2api/auth_config.json
./kiro2api get-token --type IdC
```

**输出：**
```
📂 检测到环境变量 KIRO_AUTH_TOKEN 指向文件: /etc/kiro2api/auth_config.json
✓ 读取现有配置 (2个Social token)
📝 将新token追加到配置列表
✓ 配置已保存到: /etc/kiro2api/auth_config.json
✓ 已将新token追加到配置列表 (现在共3个token，包括1个IdC)
```

**结果：** 配置文件现在同时包含 Social 和 IdC token

```json
[
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user1",
    "disabled": false
  },
  {
    "auth": "Social",
    "refreshToken": "aws_refresh_token_user2",
    "disabled": false
  },
  {
    "auth": "IdC",
    "refreshToken": "idc_refresh_token",
    "clientId": "client_id_xxx",
    "clientSecret": "client_secret_xxx",
    "disabled": false
  }
]
```

---

## 📊 工作流程图

```
┌─────────────────────────────────────────┐
│  ./kiro2api get-token [OPTIONS]         │
└────────────────┬────────────────────────┘
                 │
         ┌───────▼────────┐
         │ 执行设备授权流程 │
         └───────┬────────┘
                 │
        ┌────────▼────────┐
        │ 生成新 token    │
        │ 配置            │
        └────────┬────────┘
                 │
     ┌───────────▼──────────┐
     │ 指定了 --output 参数？│
     └───┬──────────────┬───┘
    YES │              │ NO
        │              │
   ┌────▼────┐    ┌─────▼───────────────┐
   │检查文件 │    │检查 KIRO_AUTH_TOKEN │
   │是否存在 │    │环境变量             │
   └────┬────┘    └────┬────────┬───────┘
    ┌───┴────┐      │ 指向文件  │ 无效/JSON
    │YES NO  │      │          │
    │        │    ┌─┴─┐      ┌─┴──┐
    │        │    │是 │      │否  │
    │        │    │   │      │    │
    │  ┌─────▼───┐│   │  ┌───▼──┐│
    │  │读取现有  ││   │  │只输出││
    │  │配置    ││   │  │新配置││
    │  └────┬────┘└─┬─┘  │  │  ││
    │       │       │    └──┴──┘│
    │  ┌────▼────┐ │      │     │
    │  │追加新    │ │      │     │
    │  │token    │ │      │     │
    │  └────┬────┘ │      │     │
    │       │      │      │     │
    └───┬───┴──────┴──────┘     │
        │                       │
        └───────────┬───────────┘
                    │
            ┌───────▼────────┐
            │保存/输出配置    │
            │(权限: 0600)     │
            └────────────────┘
```

---

## 🔄 完整示例：从零开始构建 3 token 池

### 第一步：获取第一个 token

```bash
# 创建新文件
mkdir -p ~/.config/kiro2api
./kiro2api get-token --output ~/.config/kiro2api/auth_config.json
# 输出：✓ Token已保存到: ~/.config/kiro2api/auth_config.json
```

### 第二步：追加第二个 token

```bash
export KIRO_AUTH_TOKEN=~/.config/kiro2api/auth_config.json
./kiro2api get-token
# 输出：✓ 已将新token追加到配置列表 (现在共2个token)
```

### 第三步：追加第三个 token（不同类型）

```bash
./kiro2api get-token --type IdC
# 输出：✓ 已将新token追加到配置列表 (现在共3个token)
```

### 第四步：验证配置

```bash
cat ~/.config/kiro2api/auth_config.json | jq '.[] | {auth: .auth, refreshToken: .refreshToken[:20]}'
# 输出：
# {
#   "auth": "Social",
#   "refreshToken": "aws_codewhisperer_r..."
# }
# {
#   "auth": "Social",
#   "refreshToken": "aws_codewhisperer_r..."
# }
# {
#   "auth": "IdC",
#   "refreshToken": "idc_refresh_token_..."
# }
```

### 第五步：启动服务

```bash
export KIRO_AUTH_TOKEN=~/.config/kiro2api/auth_config.json
export KIRO_CLIENT_TOKEN=$(openssl rand -base64 32)
./kiro2api
```

---

## ⚙️ 实现细节

### 自动检测逻辑

```go
// 如果未指定 --output，检查环境变量
if outputPath == "" {
  envPath := os.Getenv("KIRO_AUTH_TOKEN")
  if envPath != "" {
    // 检查是否为文件路径（而非 JSON 字符串）
    if fileInfo, err := os.Stat(envPath); err == nil && !fileInfo.IsDir() {
      outputPath = envPath  // 自动使用环保变量指定的路径
    }
  }
}

// 如果输出路径存在，追加到现有配置
if outputPath != "" {
  if fileExists {
    existingConfigs := readAndParseExisting(outputPath)
    config = append(existingConfigs, newTokenConfig)  // 追加而非覆盖
  }
}
```

### 文件权限

- **创建新文件：** 权限 `0600`（仅所有者可读写）
- **更新现有文件：** 保持权限 `0600`

### 错误处理

| 情况 | 处理方式 |
|------|---------|
| 文件不存在 | 创建新文件，仅包含新 token |
| 文件无法读取 | 打印警告，创建新列表 |
| JSON 解析失败 | 打印警告，覆写为新配置 |
| 写入失败 | 打印错误，退出程序 |

---

## 💡 最佳实践

1. **使用环境变量方式**（推荐生产环境）
   ```bash
   export KIRO_AUTH_TOKEN=/etc/kiro2api/auth_config.json
   ./kiro2api get-token  # 自动追加
   ```

2. **构建 token 池**
   ```bash
   # 第一次：创建文件
   ./kiro2api get-token --output config.json
   
   # 后续：自动追加
   KIRO_AUTH_TOKEN=config.json ./kiro2api get-token
   ```

3. **混合认证方式**
   ```bash
   # 获取 Social token
   KIRO_AUTH_TOKEN=config.json ./kiro2api get-token --type Social
   
   # 获取 IdC token
   KIRO_AUTH_TOKEN=config.json ./kiro2api get-token --type IdC
   ```

4. **备份配置**
   ```bash
   cp config.json config.json.backup
   KIRO_AUTH_TOKEN=config.json ./kiro2api get-token
   ```

---

## 🔐 安全考虑

1. **文件权限** - 配置文件总是以 `0600` 权限创建（仅所有者可读写）
2. **敏感信息** - Token 从不打印到日志，仅在终端输出时显示警告
3. **备份** - 在修改前考虑备份现有配置文件
4. **密钥管理** - 建议将配置文件存储在安全位置（例如 `/etc/kiro2api/`）

---

## ❓ 常见问题

### Q1: 如何覆盖现有配置？

A: 删除文件后重新运行：
```bash
rm config.json
./kiro2api get-token --output config.json
```

### Q2: 如何禁用某个 token？

A: 手动编辑 JSON，设置 `"disabled": true`：
```json
{
  "auth": "Social",
  "refreshToken": "...",
  "disabled": true
}
```

### Q3: 环境变量指定的文件不存在会怎样？

A: 系统会创建新文件，仅包含新 token。

### Q4: 支持多少个 token？

A: 理论上无限制（受 JSON 文件大小限制）。建议不超过 50 个以避免频繁轮转。

### Q5: token 追加后何时生效？

A: 立即生效。TokenManager 在下次获取 token 时会刷新缓存。

