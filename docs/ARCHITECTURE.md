# 系统架构

**最后更新**: 2026-04-22

## 1. 系统概述

ALS (Another Looking-glass Server) 是一个轻量级的 Looking-glass 服务器，用于提供网络诊断和测速功能。系统设计为无状态架构，通过 WebSocket 和 SSE 实现实时通信。

## 2. 架构分层

```
┌─────────────────────────────────────────────────┐
│                 前端层 (UI)                       │
│  Vue 3 + Vite + Naive UI + Vue I18n            │
│  - 单页应用 (SPA)                               │
│  - 多语言支持 (8 种语言)                           │
│  - 响应式设计 (支持深色模式)                     │
└─────────────────────────────────────────────────┘
                        ↓
         HTTP/WebSocket/SSE
                        ↓
┌─────────────────────────────────────────────────┐
│                 应用层 (ALS)                      │
│  Gin Web 框架 + 中间件                           │
│  - 路由分发 (route.go)                          │
│  - 会话中间件 (MiddlewareSessionOnHeader/Url)   │
│  - 控制器 (/controller)                         │
└─────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────┐
│               业务逻辑层                          │
│  - Session 管理 (client/client.go)              │
│  - 工具实现 (ping/iperf3/speedtest/shell)       │
│  - 定时任务 (timer/)                            │
└─────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────┐
│               基础设施层                          │
│  - 配置管理 (config/)                           │
│  - 静态文件服务 (embed/ui)                      │
│  - 系统命令执行 (exec.CommandContext)           │
└─────────────────────────────────────────────────┘
```

## 3. 组件详解

### 3.1 前端组件

**技术栈**:
- Vue 3 (Composition API)
- Vite 构建工具
- Naive UI 组件库
- Pinia 状态管理
- Vue I18n 国际化
- Vue3-ApexCharts 图表
- Xterm.js 终端模拟

**目录结构** (`ui/`):
```
ui/
├── src/
│   ├── components/          # Vue 组件
│   │   ├── Loading.vue     # 加载组件
│   │   ├── Information.vue # 服务器信息展示
│   │   ├── Speedtest.vue   # 测速组件
│   │   ├── Utilities.vue   # 工具集合组件
│   │   ├── TrafficDisplay.vue  # 流量显示
│   │   └── Utilities/
│   │       ├── Ping.vue    # Ping 工具
│   │       ├── IPerf3.vue  # iPerf3 工具
│   │       ├── Shell.vue   # Shell 终端
│   │       └── SpeedtestNet.vue  # Speedtest.net
│   ├── config/
│   │   └── lang.js         # 多语言配置
│   ├── locales/            # 翻译文件
│   ├── stores/
│   │   └── app.js          # 全局状态
│   ├── helper/
│   │   └── unit.js         # 工具函数
│   ├── App.vue             # 根组件
│   └── main.js             # 入口文件
├── public/
│   └── speedtest_worker.js # 测速 Web Worker
└── package.json
```

**核心特性**:
- 自动语言检测 (基于浏览器设置)
- 语言切换 (支持 8 种语言)
- 深色/浅色模式 (跟随系统)
- WebSocket 实时通信
- SSE (Server-Sent Events)

### 3.2 后端组件

**技术栈**:
- Go 1.26
- Gin Web Framework
- Gorilla WebSocket
- Cobra CLI
- PTY (伪终端)

**目录结构** (`backend/`):
```
backend/
├── main.go                  # 程序入口
├── als/                     # ALS 核心模块
│   ├── als.go              # 初始化入口
│   ├── route.go            # 路由配置
│   ├── client/             # 会话管理
│   ├── controller/         # 控制器
│   │   ├── middleware.go   # 中间件
│   │   ├── session/        # 会话处理
│   │   ├── ping/           # Ping 工具
│   │   ├── iperf3/         # iPerf3 工具
│   │   ├── speedtest/      # 测速工具
│   │   ├── shell/          # Shell 工具
│   │   └── cache/          # 缓存接口
│   └── timer/              # 定时任务
├── config/                  # 配置模块
│   ├── init.go             # 配置初始化
│   ├── load_from_env.go    # 环境变量加载
│   └── location.go         # 位置信息获取
├── http/                    # HTTP 服务
│   └── init.go             # HTTP 服务器初始化
├── fakeshell/               # Fake Shell 模块
│   ├── main.go             # Shell 入口
│   ├── menu.go             # 命令菜单
│   └── commands/           # 命令实现
└── embed/                   # 嵌入资源
    └── ui/                 # 前端静态资源
```

### 3.3 会话管理机制

**会话创建流程**:
1. 客户端 `GET /session` → 创建新会话
2. 服务器生成 UUID 作为会话 ID
3. 通过 SSE 推送会话 ID 给客户端
4. 后续请求携带会话 ID (Header 或 URL 参数)

**会话验证**:
- `MiddlewareSessionOnHeader()`: 从 Header 获取会话 ID
- `MiddlewareSessionOnUrl()`: 从 URL 参数获取会话 ID
- 检查会话是否存在且未过期 (24 小时)

**会话存储**:
- 内存存储 (`map[string]*ClientSession`)
- 自动清理过期会话 (每小时)
- 支持广播消息给所有活跃会话

### 3.4 网络工具实现

#### Ping 工具
- 位置：`backend/als/controller/ping/ping.go`
- 实现：使用 `go-ping` 库
- 功能：支持 IPv4/IPv6

#### iPerf3 工具
- 位置：`backend/als/controller/iperf3/iperf3.go`
- 实现：启动 iPerf3 服务器进程
- 端口范围：30000-31000 (可配置)

#### Speedtest 工具
**LibreSpeed**:
- 位置：`backend/als/controller/speedtest/librespeed.go`
- 实现：基于 LibreSpeed 项目
- 功能：下载/上传测试

**Speedtest.net**:
- 位置：`backend/als/controller/speedtest/speedtest_cli.go`
- 实现：调用 speedtest-cli 工具

**文件测速**:
- 位置：`backend/als/controller/speedtest/fakefile.go`
- 实现：动态生成指定大小的文件

#### Shell 工具
- 位置：`backend/als/controller/shell/shell.go`
- 实现：基于 WebSocket 的 PTY 终端
- 命令白名单：ping, traceroute, mtr, speedtest
- 参数过滤：危险参数拦截

## 4. 接口协议

### 4.1 HTTP API

| 端点 | 方法 | 描述 | 认证 |
|------|------|------|------|
| `/session` | GET | 创建会话 (SSE) | 无 |
| `/method/iperf3/server` | GET | iPerf3 服务器信息 | Session Header |
| `/method/ping` | GET | Ping 测试 | Session Header |
| `/method/speedtest_dot_net` | GET | Speedtest.net | Session Header |
| `/method/cache/interfaces` | GET | 网卡接口缓存 | Session Header |
| `/session/:session/shell` | GET | WebSocket Shell | Session URL 参数 |
| `/session/:session/speedtest/file/:filename` | GET | 文件测速 | Session URL 参数 |
| `/session/:session/speedtest/download` | GET | LibreSpeed 下载 | Session URL 参数 |
| `/session/:session/speedtest/upload` | POST | LibreSpeed 上传 | Session URL 参数 |

### 4.2 WebSocket

**Shell 连接**:
- URL: `ws(s)://host/session/<session-id>/shell`
- 协议：WebSocket
- 消息格式:
  - `1<data>`: 输入数据
  - `2<h>;<w>`: 窗口大小调整

## 5. 配置系统

### 5.1 环境变量

| 变量 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `LISTEN_IP` | string | 0.0.0.0 | 监听 IP |
| `HTTP_PORT` | string | 80 | 监听端口 |
| `LOCATION` | string | (自动获取) | 服务器位置 |
| `PUBLIC_IPV4` | string | (自动获取) | 公网 IPv4 |
| `PUBLIC_IPV6` | string | (自动获取) | 公网 IPv6 |
| `SPEEDTEST_FILE_LIST` | string | "1MB 10MB 100MB 1GB" | 测速文件列表 |
| `SPONSOR_MESSAGE` | string | "" | 赞助商信息 |
| `DISPLAY_TRAFFIC` | bool | true | 流量显示开关 |
| `ENABLE_SPEEDTEST` | bool | true | LibreSpeed 开关 |
| `UTILITIES_PING` | bool | true | Ping 开关 |
| `UTILITIES_FAKESHELL` | bool | true | Shell 开关 |
| `UTILITIES_IPERF3` | bool | true | iPerf3 开关 |
| `UTILITIES_IPERF3_PORT_MIN` | int | 30000 | iPerf3 起始端口 |
| `UTILITIES_IPERF3_PORT_MAX` | int | 31000 | iPerf3 结束端口 |
| `UTILITIES_MTR` | bool | true | MTR 开关 |
| `UTILITIES_TRACEROUTE` | bool | true | Traceroute 开关 |

### 5.2 启动流程

```
main()
├── flag.Parse()
├── --shell? 
│   ├── config.IsInternalCall = true
│   ├── config.Load()
│   └── fakeshell.HandleConsole()
└── 正常模式
    ├── config.LoadWebConfig()
    ├── als.Init()
    │   ├── alsHttp.CreateServer()
    │   ├── SetupHttpRoute()
    │   ├── 启动定时任务
    │   └── aHttp.Start()
```

## 6. 部署架构

### Docker 部署

```
┌─────────────────────────────────────┐
│  Docker 容器                        │
│  ┌───────────────────────────────┐  │
│  │  /bin/als (主程序)            │  │
│  │  - 嵌入前端静态资源           │  │
│  │  - 监听 80 端口               │  │
│  └───────────────────────────────┘  │
│  系统工具层：                        │
│  - ping, iperf3, mtr, traceroute   │
└─────────────────────────────────────┘
```

**Dockerfile 构建阶段**:
1. `builder_node_js_cache`: 安装 Node.js 依赖
2. `builder_node_js`: 构建前端
3. `builder_golang`: 编译 Go 后端 + 嵌入前端
4. `builder_env`: 安装系统工具
5. `final`: 合并产物

### 网络要求

- **网络模式**: Host 模式 (推荐) 或 Bridge 模式
- **端口**: 80 (可配置)
- **容器能力**: 需要网络相关权限执行 ping 等工具

## 7. CI/CD 架构

### GitHub Actions 工作流

**CI** (`ci.yml`):
- 触发：PR 和 push
- 步骤：
  1. 构建前端 (npm run build)
  2. 下载前端到后端嵌入目录
  3. Go 测试 (go test)
  4. Go 构建 (go build)

**Docker Image** (`docker-image.yml`):
- 触发：Tag 推送
- 步骤：
  1. 构建前端
  2. 构建多架构 Docker 镜像 (amd64, arm64)
  3. 推送到 Docker Hub
  4. 生成 SBOM 和 provenance

**Release** (`release.yml`):
- 触发：Tag 推送
- 步骤：
  1. 构建前端
  2. 交叉编译多平台二进制 (linux/darwin/windows × amd64/arm64)
  3. 创建 GitHub Release

## 8. 安全考虑

### 8.1 会话安全
- 会话 ID 使用 UUID v4
- 24 小时过期机制
- 内存隔离不同会话

### 8.2 Shell 安全
- 命令白名单机制
- 危险参数过滤 (`ping -f`)
- 不可运行任意命令
- 可通过环境变量禁用

### 8.3 CORS
- 检查 WebSocket 连接 Origin
- 仅允许同源连接

### 8.4 依赖安全
- 固定依赖版本
- 定期安全更新

## 9. 性能优化

### 前端
- Vite 构建 (HMR 开发，按需打包)
- 组件懒加载 (defineAsyncComponent)
- WebSocket 复用连接

### 后端
- Gin Release Mode
- 内存缓存 (接口信息、系统资源)
- 定时清理过期会话
- 上下文感知 (context.Context 取消)

## 10. 监控与日志

### 日志
- Go 标准日志 (`log.Default()`)
- Gin 访问日志 (Release Mode 简化)

### 健康检查
- Docker HEALTHCHECK
- 每 30 秒检测 `/`
- 超时 5 秒，重试 3 次

### 资源监控
- 实时内存使用 (显示在页脚)
- 网卡流量广播 (1 秒间隔)
- 系统资源更新 (定时器)
