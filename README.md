# 炸金花游戏 (AI Jinhua)

一个基于Go语言开发的炸金花游戏，支持多人在线对战。本项目已升级为 **WebSocket** 协议，支持在任何现代云平台（如 Render, Fly.io）上轻松部署，并支持 SSL/TLS 加密链接。

## ✨ 功能特点

- 🎮 **完整的炸金花游戏规则**：支持闷牌、看牌、跟注、加注、弃牌、比牌、全押等操作
- 👥 **多人在线对战**：支持多玩家同时在线游戏
- 🏠 **房间系统**：房主创建房间，其他玩家加入
- 💬 **实时聊天**：游戏中支持玩家聊天
- 📊 **游戏统计**：显示每局获胜者和获胜金额
- 🌐 **WebSocket通信**：支持 ws:// 和 wss:// 协议，自动处理 SSL 连接
- 💻 **跨平台支持**：Windows / Linux / macOS 完美运行

## 🚀 快速开始

### 1. 环境要求

- Go 1.18+
- 依赖库：`github.com/gorilla/websocket`

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 编译运行

**Windows**

```bash
# 编译
go build -o AI_jinhua.exe .

# 启动服务器
.\AI_jinhua.exe server

# 启动客户端
.\AI_jinhua.exe client
```

**macOS / Linux**

```bash
# 编译
go build -o AI_jinhua .

# 赋予权限
chmod +x AI_jinhua

# 启动服务器
./AI_jinhua server

# 启动客户端
./AI_jinhua client
```

## ☁️ 部署指南

本项目针对 **Render** 进行了专门优化，可直接作为 Web Service 免费部署。

1. **Fork** 本仓库到您的 GitHub。
2. 在 [Render Dashboard](https://dashboard.render.com/) 创建新的 **Web Service**。
3. 连接您的仓库，并填写以下配置：
   - **Build Command**: `go build -o app .`
   - **Start Command**: `./app server`
4. 部署完成后，您将获得一个 HTTPS 地址（如 `https://your-app.onrender.com`）。

客户端连接时，直接输入该 HTTPS 地址即可（程序会自动转换为 WebSocket 安全连接）。

详细部署教程请参考 [DEPLOY_RENDER.md](DEPLOY_RENDER.md)。

## 🎮 游戏操作说明

| 动作 | 快捷键 | 说明 |
|------|--------|------|
| **看牌** | `k` | 查看自己的手牌 |
| **闷注** | `b [金额]` | 不看牌直接下注，默认跟底注 |
| **跟注** | `c` | 跟上当前的单注金额 |
| **加注** | `r [金额]` | 提高单注金额 |
| **全押** | `allin` | 押上所有筹码 |
| **弃牌** | `f` | 放弃当前手牌，退出本局 |
| **比牌** | `v [ID]` | 与指定ID的玩家比牌 |
| **退出** | `exit` | 退出房间或游戏 |

## 🛠️ 技术栈

- **语言**：Go 1.18+
- **网络协议**：WebSocket (基于 HTTP/HTTPS)
- **依赖库**：`gorilla/websocket`
- **并发模型**：Goroutine + Channel + Select
- **数据交互**：JSON

## 📂 项目结构

```
├── main.go            # 程序入口 (CLI参数处理)
├── server.go          # WebSocket服务器实现
├── client.go          # 交互式客户端实现
├── room.go            # 房间与游戏流程控制
├── player.go          # 玩家状态管理
├── card.go            # 扑克牌定义
├── deck.go            # 洗牌与发牌逻辑
├── judge.go           # 牌型大小判断算法
├── protocol.go        # 通信协议定义
└── DEPLOY_RENDER.md   # Render部署文档
```

## 📜 更新日志

### v1.1.0 (2026-01-15)
- 🔄 **重构网络层**：从纯 TCP 迁移至 WebSocket 协议
- ☁️ **云原生支持**：完美支持 Render 等 PaaS 平台部署
- 🔒 **安全升级**：支持 WSS (WebSocket Secure) 加密连接
- 🐛 **Bug修复**：修复了构建命令在部分环境下 undefined 的问题

### v1.0.0 (2026-01-14)
- ✨ 初始版本发布，支持 TCP 局域网对战

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License
