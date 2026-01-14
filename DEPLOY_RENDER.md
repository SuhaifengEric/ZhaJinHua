# 炸金花游戏 Render 部署指南

本指南将帮助您将炸金花游戏服务端部署到 Render 平台，让您的朋友可以通过互联网连接到您的游戏服务器。

## 一、准备工作

### 1. 代码准备

确保您的代码已经：
- 上传到 GitHub 仓库
- 支持从环境变量获取端口（我们已经帮您修改好了）
- 支持通过命令行参数直接启动服务器或客户端

### 2. 技术栈

- **语言**：Go 1.18+
- **网络协议**：TCP
- **部署平台**：Render

## 二、Render 部署步骤

### 步骤 1：创建 Render 账户

1. 访问 [Render 官网](https://render.com/)，点击 "Sign Up" 注册账号
2. 可以使用 GitHub 账号直接登录，方便后续连接仓库

### 步骤 2：连接 GitHub 仓库

1. 登录后，点击顶部导航栏的 "Dashboard"
2. 点击 "New" → "Web Service"
3. 在 "Connect a repository" 部分，选择您的炸金花游戏仓库
4. 如果没有看到您的仓库，点击 "Configure account" 并授权 Render 访问您的 GitHub 仓库

### 步骤 3：配置部署选项

在配置页面，填写以下信息：

| 配置项 | 取值 | 说明 |
|--------|------|------|
| **Name** | `AI-jinhua-server` | 服务名称，可自定义 |
| **Runtime** | `Go` | 选择 Go 运行时 |
| **Build Command** | `go build -o server main.go` | 编译命令 |
| **Start Command** | `./server` | 启动命令 |
| **Branch** | `main` | 要部署的分支 |
| **Region** | `Singapore` | 选择离您近的区域，如新加坡或东京 |
| **Instance Type** | `Free` | 免费实例，适合小型应用 |

### 步骤 4：配置环境变量

在 "Environment Variables" 部分，添加以下环境变量（如果需要）：

| 变量名 | 取值 | 说明 |
|--------|------|------|
| `PORT` | 留空 | Render 会自动分配端口 |

### 步骤 5：部署服务

1. 确认配置信息无误后，点击 "Create Web Service"
2. Render 会开始构建和部署您的服务
3. 部署过程需要几分钟时间，您可以在 "Events" 标签页查看部署日志

### 步骤 6：获取服务地址

部署成功后：

1. 进入服务详情页
2. 在页面顶部，您会看到服务的公网地址，格式为：`https://ai-jinhua-server.onrender.com`
3. 记录这个地址，客户端连接时需要使用

### 步骤 7：配置 TCP 端口（重要！）

由于炸金花游戏使用 TCP 协议通信，您需要配置 TCP 端口：

1. 在服务详情页，点击 "Settings"
2. 滚动到 "Network" 部分
3. 开启 "TCP Port"
4. Render 会为您分配一个 TCP 端口号（如：`12345`）
5. 最终客户端连接地址格式为：`ai-jinhua-server.onrender.com:12345`

## 三、客户端连接

### 方式 1：使用命令行参数直接启动客户端（推荐）

1. 下载编译好的可执行文件
2. 运行客户端：
   ```bash
   # Windows
   AI_jinhua.exe client
   
   # Linux/macOS
   ./AI_jinhua client
   ```
3. 输入服务器地址，格式为：`ai-jinhua-server.onrender.com:12345`
4. 输入昵称，开始游戏

### 方式 2：使用交互式选择

1. 运行主程序：
   ```bash
   # Windows
   AI_jinhua.exe
   
   # Linux/macOS
   ./AI_jinhua
   ```
2. 输入 `2` 选择客户端模式
3. 输入服务器地址
4. 输入昵称，开始游戏

## 四、代码编译

### 编译主程序

```bash
# 编译主程序（支持命令行参数启动服务器或客户端）
go build -o AI_jinhua.exe main.go
```

### 服务器启动方式

```bash
# 使用命令行参数直接启动服务器
# Windows
AI_jinhua.exe server

# Linux/macOS
./AI_jinhua server
```

### 跨平台编译

```bash
# Windows 编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o AI_jinhua_linux main.go

# Windows 编译 macOS 版本
GOOS=darwin GOARCH=amd64 go build -o AI_jinhua_mac main.go
```

## 五、目录结构

```
├── main.go          # 主程序入口（支持命令行参数启动服务器或客户端）
├── server.go        # 服务器实现
├── client.go        # 客户端实现
├── room.go          # 房间管理
├── player.go        # 玩家管理
├── card.go          # 卡牌实现
├── judge.go         # 手牌分析
├── protocol.go      # 通信协议
└── DEPLOY_RENDER.md # 部署指南
```

## 六、注意事项

### 1. 免费实例限制

- **运行时间**：免费实例每月有 750 小时的运行时间限制
- **闲置休眠**：25 分钟无请求会自动休眠
- **并发连接**：适合 3-5 人同时游戏

### 2. TCP 连接注意事项

- 确保客户端和服务端使用相同的 TCP 协议
- 如果连接失败，检查防火墙是否允许 TCP 端口访问
- Render 分配的 TCP 端口可能会在重新部署后变化

### 3. 服务重启

- 代码更新后，Render 会自动重新部署
- 重新部署会导致 TCP 端口变化，需要告知玩家新的连接地址

### 4. 日志查看

- 在 Render 服务详情页的 "Logs" 标签页可以查看服务器日志
- 可以通过日志排查连接问题

## 七、常见问题

### Q: 客户端连接失败怎么办？

A: 检查以下几点：
1. 服务器是否正在运行（查看 Render 服务状态）
2. TCP 端口是否正确配置
3. 输入的服务器地址格式是否正确
4. 网络是否允许 TCP 连接

### Q: 为什么服务会自动停止？

A: 免费实例在 25 分钟无请求后会自动休眠，可以：
1. 升级到付费实例
2. 使用定时任务保持服务活跃
3. 重新访问服务页面唤醒服务

### Q: 如何查看服务器日志？

A: 在 Render 服务详情页的 "Logs" 标签页可以查看完整的服务器日志。

### Q: 如何更新游戏代码？

A: 将代码推送到 GitHub 仓库后，Render 会自动检测并重新部署服务。

## 八、联系方式

如有问题或建议，欢迎联系：
- 项目地址：https://github.com/yourusername/AI_jinhua
- 邮箱：your.email@example.com

---

**祝您游戏愉快！** 🎉