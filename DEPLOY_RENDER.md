# 炸金花游戏 Render 部署指南

本指南将帮助您将炸金花游戏服务端部署到 Render 平台，让您的朋友可以通过互联网连接到您的游戏服务器。

> **注意**：本项目使用纯 TCP 协议通信。在 Render 上部署时，请确保您了解 Render 对非 HTTP 服务的支持情况（通常需要使用 Docker 部署或特定的 TCP 服务配置）。

## 一、准备工作

### 1. 代码准备

确保您的代码已经：
- 上传到 GitHub 仓库
- **关键点**：`main.go` 已更新，支持通过命令行参数 `server` 启动服务器模式（无需交互）。

### 2. 技术栈

- **语言**：Go 1.18+
- **网络协议**：TCP
- **部署平台**：Render (Linux 环境)

## 二、Render 部署步骤

### 步骤 1：创建 Render 账户

1. 访问 [Render 官网](https://render.com/)，注册并登录。
2. 建议使用 GitHub 账号登录，以便直接访问仓库。

### 步骤 2：创建 Web Service

1. 在 Dashboard 点击 **"New +"** 按钮，选择 **"Web Service"**。
2. 连接您的 GitHub 仓库。

### 步骤 3：配置部署选项

请严格按照以下表格填写配置，特别是 **Start Command**：

| 配置项 | 推荐值 | 说明 |
|--------|------|------|
| **Name** | `AI-jinhua-server` | 服务名称 |
| **Language** | `Go` | 选择运行环境 |
| **Branch** | `main` | 部署分支 |
| **Region** | `Singapore` | 选择距离玩家较近的节点（如新加坡或东京） |
| **Build Command** | `go build -o app .` | 编译当前目录下所有文件 |
| **Start Command** | `./app server` | **重要**：必须带 `server` 参数以非交互模式启动 |
| **Instance Type** | `Free` | 免费实例（注意会有休眠机制） |

### 步骤 4：环境变量

Render 会自动注入 `PORT` 环境变量，代码中 `server.go` 已适配：
```go
port := os.Getenv("PORT")
```
因此**不需要**手动设置 PORT 变量，除非您有特殊需求。

### 步骤 5：端口配置

Render 会自动分配一个 HTTPS 域名（如 `https://your-app.onrender.com`）。

1. **健康检查**：代码中已添加 `/` 根路径的 HTTP 处理函数，Render 的健康检查会自动通过。
2. **WebSocket 路径**：游戏通信使用 `/ws` 路径。
3. **无需额外配置**：由于 WebSocket 握手基于 HTTP，您不需要在 Render 上进行任何特殊的 TCP 端口配置，直接使用默认的 Web Service 即可。

## 三、客户端连接指南

部署成功并获得服务器地址（例如 `https://ai-jinhua-server.onrender.com`）后，玩家可以通过以下方式连接。

**注意**：客户端会自动处理 `https://` 到 `wss://` 的转换。您只需要复制 Render 提供的 URL 即可。

### 方式 1：命令行参数启动（推荐）

直接通过命令行参数指定服务器地址。

**Windows (PowerShell)**
```powershell
# 格式: .\AI_jinhua.exe client
.\AI_jinhua.exe client
# 输入: https://your-app.onrender.com
```

**macOS / Linux**
```bash
# 格式: ./AI_jinhua client
./AI_jinhua client
# 输入: https://your-app.onrender.com
```

### 方式 2：交互式启动

1. 运行程序 `AI_jinhua` (或 `.exe`)。
2. 输入 `2` 选择 **交互式客户端**。
3. 当提示 `请输入服务器地址` 时，粘贴 Render 提供的完整 URL（如 `https://ai-jinhua-server.onrender.com`）。

## 四、本地开发与编译

### 编译命令

```bash
# Windows
go build -o AI_jinhua.exe main.go

# macOS / Linux
go build -o AI_jinhua main.go
```

### 本地测试

**启动服务器**
```bash
# Windows
.\AI_jinhua.exe server

# macOS / Linux
./AI_jinhua server
```

**启动客户端**
```bash
# Windows
.\AI_jinhua.exe client

# macOS / Linux
./AI_jinhua client
```

## 五、常见问题排查

### 1. 部署显示 "Build Successful" 但服务无法访问
- **原因**：启动命令错误，导致程序进入了交互式等待输入模式。
- **解决**：检查 Start Command 是否为 `./app server`（确保带上了 `server` 参数）。

### 2. 客户端连接超时
- **原因**：Render 的免费 Web Service 仅支持 HTTP 流量，可能拦截了纯 TCP 连接。
- **解决**：
  - 尝试使用 Render 的 Docker 部署并配置 TCP 端口。
  - 或者迁移到支持 TCP 的平台（如 Fly.io, Railway, AWS Lightsail）。

### 3. 服务自动休眠
- Render 免费实例在 15 分钟无活动后会自动休眠。客户端连接时可能需要等待几十秒唤醒服务。

## 六、维护与更新

- **更新代码**：只需将代码 push 到 GitHub 的 `main` 分支，Render 会自动触发重新构建和部署。
- **查看日志**：在 Render Dashboard 的 **Logs** 页面可以查看服务器运行日志（如玩家连接、报错信息）。
