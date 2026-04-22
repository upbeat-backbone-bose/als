# 后端核心模块

**模块路径**: `backend/`

**最后更新**: 2026-04-22

## 1. 模块概述

ALS 的后端是使用 Go 1.26 构建的 HTTP 服务器，提供网络诊断和测速功能。采用 Gin Web 框架，支持 WebSocket 和 SSE 通信。

**核心职责**:
- HTTP 路由和中间件
- 网络工具执行
- 会话管理
- 配置文件加载
- 静态资源服务

## 2. 启动流程

### 2.1 入口点

**文件**: `backend/main.go`

```go
package main

import (
    "flag"
    "github.com/samlm0/als/v2/als"
    "github.com/samlm0/als/v2/config"
    "github.com/samlm0/als/v2/fakeshell"
)

var shell = flag.Bool("shell", false, "Start as fake shell")

func main() {
    flag.Parse()
    
    if *shell {
        // Fake Shell 模式
        config.IsInternalCall = true
        config.Load()
        fakeshell.HandleConsole()
        return
    }

    // HTTP 服务器模式
    config.LoadWebConfig()
    als.Init()
}
```

### 2.2 启动分支

**分支 1 - Fake Shell**:
```bash
./als --shell
```

执行流程：
1. 设置 `IsInternalCall = true` (跳过日志前缀)
2. 加载配置
3. 启动交互式 Shell 菜单

**分支 2 - HTTP 服务器**:
```bash
./als
```

执行流程：
1. 调用 `config.LoadWebConfig()`
2. 调用 `als.Init()`

## 3. 核心子模块

### 3.1 ALS 模块

**位置**: `backend/als/`

**启动初始化** (`backend/als/als.go`):

```go
func Init() {
    aHttp := alsHttp.CreateServer()

    log.Default().Println(
        "Listen on: " + config.Config.ListenHost + ":" + config.Config.ListenPort
    )
    aHttp.SetListen(config.Config.ListenHost + ":" + config.Config.ListenPort)

    // 配置路由
    SetupHttpRoute(aHttp.GetEngine())

    // 启动后台任务
    if config.Config.FeatureIfaceTraffic {
        go timer.SetupInterfaceBroadcast()
    }
    go timer.UpdateSystemResource()
    go client.HandleQueue()
    go cleanupExpiredClients()

    // 启动 HTTP 服务器
    aHttp.Start()
}

func cleanupExpiredClients() {
    ticker := time.NewTicker(1 * time.Hour)
    for {
        <-ticker.C
        removed := client.RemoveExpiredClients()
        if removed > 0 {
            log.Default().Printf(
                "Cleaned up %d expired sessions\n", removed
            )
        }
    }
}
```

**职责**:
- 创建 HTTP 服务器
- 注册路由
- 启动定时任务

### 3.2 路由模块

**位置**: `backend/als/route.go`

**路由组织**:

```go
func SetupHttpRoute(e *gin.Engine) {
    // 1. Session 端点 (SSE)
    e.GET("/session", session.Handle)

    // 2. v1 API 组 (Session Header)
    v1 := e.Group("/method", controller.MiddlewareSessionOnHeader())
    {
        if config.Config.FeatureIperf3 {
            v1.GET("/iperf3/server", iperf3.Handle)
        }
        if config.Config.FeaturePing {
            v1.GET("/ping", ping.Handle)
        }
        if config.Config.FeatureSpeedtestDotNet {
            v1.GET("/speedtest_dot_net", speedtest.HandleSpeedtestDotNet)
        }
        if config.Config.FeatureIfaceTraffic {
            v1.GET("/cache/interfaces", cache.UpdateInterfaceCache)
        }
    }

    // 3. Session 相关组 (URL 参数)
    session := e.Group("/session/:session", controller.MiddlewareSessionOnUrl())
    {
        if config.Config.FeatureShell {
            session.GET("/shell", shell.HandleNewShell)
        }
    }

    speedtestRoute := session.Group("/speedtest", controller.MiddlewareSessionOnUrl())
    {
        if config.Config.FeatureFileSpeedtest {
            speedtestRoute.GET("/file/:filename", speedtest.HandleFakeFile)
        }
        if config.Config.FeatureLibrespeed {
            speedtestRoute.GET("/download", speedtest.HandleDownload)
            speedtestRoute.POST("/upload", speedtest.HandleUpload)
        }
    }

    // 4. 静态资源
    e.Any("/assets/:filename", handleStatisFile)
    e.GET("/", handleStatisFile)
    e.GET("/speedtest_worker.js", handleStatisFile)
    e.GET("/favicon.ico", handleStatisFile)
}
```

**路由图**:

```
/:session/shell
├── /session                      # 创建会话
├── /method                       # 需要 Session Header
│   ├── /iperf3/server
│   ├── /ping
│   ├── /speedtest_dot_net
│   └── /cache/interfaces
└── /session/:session             # 需要 Session URL 参数
    ├── /shell
    └── /speedtest
        ├── /file/:filename
        ├── /download
        └── /upload
```

### 3.3 中间件模块

**位置**: `backend/als/controller/middleware.go`

**Session Header 验证**:
```go
func MiddlewareSessionOnHeader() gin.HandlerFunc {
    return func(c *gin.Context) {
        sessionId := c.GetHeader("session")
        clientSession, ok := client.GetClient(sessionId)
        if !ok {
            c.JSON(400, gin.H{
                "success": false,
                "error":   "Invalid session",
            })
            c.Abort()
            return
        }
        c.Set("clientSession", clientSession)
        c.Next()
    }
}
```

**Session URL 参数验证**:
```go
func MiddlewareSessionOnUrl() gin.HandlerFunc {
    return func(c *gin.Context) {
        sessionId := c.Param("session")
        clientSession, ok := client.GetClient(sessionId)
        if !ok {
            c.JSON(400, gin.H{
                "success": false,
                "error":   "Invalid session",
            })
            c.Abort()
            return
        }
        c.Set("clientSession", clientSession)
        c.Next()
    }
}
```

**功能**:
- 解析请求中的 Session ID
- 验证 Session 是否存在和过期
- 将 Session 存入 Context
- 终止无效请求

## 4. 控制器模块

### 4.1 目录结构

```
backend/als/controller/
├── middleware.go       # 中间件
├── session/
│   └── session.go      # 会话管理
├── ping/
│   └── ping.go         # Ping 工具
├── iperf3/
│   └── iperf3.go       # iPerf3 工具
├── speedtest/
│   ├── speedtest_cli.go    # Speedtest.net
│   ├── fakefile.go         # 静态文件测速
│   └── librespeed.go       # LibreSpeed
├── shell/
│   └── shell.go            # Shell 工具
└── cache/
    └── interface.go        # 网卡缓存
```

### 4.2 Session 控制器

**文件**: `backend/als/controller/session/session.go`

**职责**:
- 创建新会话
- 分配 UUID
- SSE 推送配置

**关键代码**:
```go
func Handle(c *gin.Context) {
    uuid := uuid.New().String()
    channel := make(chan *client.Message, 64)
    clientSession := &client.ClientSession{
        Channel:   channel,
        CreatedAt: time.Now(),
    }
    client.AddClient(uuid, clientSession)

    ctx, cancel := context.WithCancel(c.Request.Context())
    clientSession.SetContext(ctx)
    defer func() {
        cancel()
        client.RemoveClient(uuid)
    }()

    c.Writer.Header().Set("Content-Type", "text/event-stream")
    c.Writer.Header().Set("Cache-Control", "no-cache")
    c.SSEvent("SessionId", uuid)
    
    _config := &sessionConfig{
        ALSConfig: *config.Config,
        ClientIP:  c.ClientIP(),
    }
    configJson, _ := json.Marshal(_config)
    c.SSEvent("Config", string(configJson))
    
    // 持续推送消息
    for {
        select {
        case <-ctx.Done():
            return
        case msg, ok := <-channel:
            if !ok { return }
            c.SSEvent(msg.Name, msg.Content)
        }
    }
}
```

### 4.3 Ping 控制器

**文件**: `backend/als/controller/ping/ping.go`

**实现**:
```go
func Handle(c *gin.Context) {
    target := c.Query("target")
    count := c.DefaultQuery("count", "4")
    interval := c.DefaultQuery("interval", "1000")

    c.Writer.Header().Set("Content-Type", "text/plain")
    
    cmd := exec.Command("ping", "-c", count, "-i", interval, target)
    cmd.Stdout = c.Writer
    cmd.Stderr = c.Writer
    
    if err := cmd.Run(); err != nil {
        c.String(http.StatusInternalServerError, err.Error())
    }
}
```

**关键点**:
- 获取目标地址
- 设置输出到响应流
- 实时传输 Ping 结果

### 4.4 iPerf3 控制器

**文件**: `backend/als/controller/iperf3/iperf3.go`

**核心功能**:
- 动态分配端口
- 启动 iPerf3 服务器
- 返回连接信息

```go
func Handle(c *gin.Context) {
    port := allocatePort()  // 在 30000-31000 范围内分配
    
    // 启动 iPerf3 服务器
    cmd := exec.Command("iperf3", "-s", "-p", strconv.Itoa(port))
    cmd.Start()

    c.JSON(http.StatusOK, gin.H{
        "server": config.Config.PublicIPv4,
        "port":   port,
    })
}
```

### 4.5 Speedtest 控制器

**三个实现**:

1. **Speedtest.net** (`speedtest_cli.go`):
   ```go
   func HandleSpeedtestDotNet(c *gin.Context) {
       cmd := exec.Command("speedtest", "--format=json")
       output, _ := cmd.Output()
       c.Data(http.StatusOK, "application/json", output)
   }
   ```

2. **静态文件测速** (`fakefile.go`):
   ```go
   func HandleFakeFile(c *gin.Context) {
       filename := c.Param("filename")
       size := parseSize(filename)  // 解析 "1GB" -> 1073741824
       
       // 动态生成全零数据
       c.Stream(func(w io.Writer) bool {
           _, err := w.Write(zeroBuff)
           return err == nil
       })
   }
   ```

3. **LibreSpeed** (`librespeed.go`):
   ```go
   func HandleDownload(c *gin.Context) {
       // 生成随机数据
       randomData := generateRandomData()
       c.Data(http.StatusOK, "application/octet-stream", randomData)
   }

   func HandleUpload(c *gin.Context) {
       // 接收数据但不保存
       io.Copy(io.Discard, c.Request.Body)
       c.Status(http.StatusOK)
   }
   ```

### 4.6 Shell 控制器

**文件**: `backend/als/controller/shell/shell.go`

**职责**:
- WebSocket 升级
- PTY 启动
- 双向转发

**详见**: [控制台.md](../专有概念/控制台.md)

## 5. 客户端子模块

**位置**: `backend/als/client/`

### 5.1 会话存储

```go
var (
    clientsMu sync.RWMutex
    Clients   = make(map[string]*ClientSession)
)

type ClientSession struct {
    Channel    chan *Message
    ctx        context.Context
    CreatedAt  time.Time
    cancelFunc context.CancelFunc
}
```

### 5.2 核心函数

**添加会话**:
```go
func AddClient(id string, session *ClientSession) {
    clientsMu.Lock()
    defer clientsMu.Unlock()
    Clients[id] = session
}
```

**获取会话**:
```go
func GetClient(id string) (*ClientSession, bool) {
    clientsMu.RLock()
    defer clientsMu.RUnlock()
    session, ok := Clients[id]
    if ok && time.Since(session.CreatedAt) > sessionExpireDuration {
        return nil, false
    }
    return session, ok
}
```

**移除会话**:
```go
func RemoveClient(id string) {
    clientsMu.Lock()
    defer clientsMu.Unlock()
    if client, ok := Clients[id]; ok && client.cancelFunc != nil {
        client.cancelFunc()
    }
    delete(Clients, id)
}
```

**消息广播**:
```go
func BroadCastMessage(name string, content string) {
    msg := &Message{
        Name:    name,
        Content: content,
    }
    for _, client := range SnapshotClients() {
        client.TrySend(msg)
    }
}
```

**队列处理**:
```go
func HandleQueue() {
    ticker := time.NewTicker(1 * time.Second)
    for range ticker.C {
        // 处理排队消息
    }
}
```

## 6. 定时任务模块

**位置**: `backend/als/timer/`

### 6.1 系统资源更新

**文件**: `backend/als/timer/system.go`

```go
func UpdateSystemResource() {
    ticker := time.NewTicker(1 * time.Second)
    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        memoryUsage := fmt.Sprintf("%.1f%%", 
            float64(m.Alloc)/float64(m.Sys)*100)
        
        client.BroadCastMessage("SystemResource", memoryUsage)
    }
}
```

### 6.2 网卡流量更新

**文件**: `backend/als/timer/interface_traffic.go`

```go
func SetupInterfaceBroadcast() {
    ticker := time.NewTicker(1 * time.Second)
    for range ticker.C {
        interfaces, _ := net.Interfaces()
        var traffic []InterfaceTraffic
        
        for _, iface := range interfaces {
            rx, tx := getTraffic(iface.Name)
            traffic = append(traffic, InterfaceTraffic{
                Name:      iface.Name,
                RxBytes:   rx,
                TxBytes:   tx,
                Timestamp: time.Now(),
            })
        }
        
        client.BroadCastMessage("InterfaceTraffic", 
            json.Marshal(traffic))
    }
}
```

## 7. 配置模块

**位置**: `backend/config/`

**详见**: [config.md](./config.md)

## 8. HTTP 服务模块

**位置**: `backend/http/`

### 8.1 服务器创建

**文件**: `backend/http/init.go`

```go
type Server struct {
    engine *gin.Engine
    listen string
}

func CreateServer() *Server {
    gin.SetMode(gin.ReleaseMode)
    e := &Server{
        engine: gin.Default(),
        listen: ":8080",
    }
    return e
}

func (e *Server) Start() error {
    return e.engine.Run(e.listen)
}
```

**功能**:
- 设置 Gin 为 Release 模式
- 创建默认引擎（包含 Logger、Recovery 中间件）
- 设置监听地址

## 9. FakeShell 模块

**位置**: `backend/fakeshell/`

**详见**: [fakeshell.md](./fakeshell.md)

## 10. 嵌入资源模块

**位置**: `backend/embed/`

### 10.1 静态文件嵌入

**文件**: `backend/embed/ui.go`

```go
//go:embed ui
var UIStaticFiles embed.FS
```

**说明**:
- 使用 Go embed 特性
- 编译时将前端构建物嵌入
- 无需额外文件部署

### 10.2 文件服务

**路由** (`backend/als/route.go`):

```go
func handleStatisFile(filePath string, c *gin.Context) {
    uiFs := iEmbed.UIStaticFiles
    subFs, _ := fs.Sub(uiFs, "ui")
    httpFs := http.FileServer(http.FS(subFs))
    
    _, err := fs.ReadFile(subFs, filePath)
    if err != nil {
        c.String(404, "Not found")
        c.Abort()
        return
    }
    httpFs.ServeHTTP(c.Writer, c.Request)
}
```

**流程**:
1. 从 embed.FS 创建子文件系统
2. 创建 HTTP 文件服务器
3. 检查文件是否存在
4. 如果存在则服务

## 11. 依赖管理

### 11.1 Go 模块配置

**文件**: `backend/go.mod`

```go
module github.com/samlm0/als/v2

go 1.26

require (
    github.com/creack/pty v1.1.24          // PTY 支持
    github.com/gin-gonic/gin v1.12.0       // Web 框架
    github.com/google/uuid v1.6.0          // UUID 生成
    github.com/gorilla/websocket v1.5.3    // WebSocket
    github.com/reeflective/console v0.1.25 // 交互式 CLI
    github.com/spf13/cobra v1.10.2         // CLI 工具
    github.com/samlm0/go-ping v0.1.0       // Ping 库
    github.com/miekg/dns v1.1.72           // DNS 解析
)
```

### 11.2 关键依赖说明

**Gin**:
- Web 框架
- 高性能 HTTP 路由
- 中间件支持

**Gorilla WebSocket**:
- WebSocket 实现
- 标准 API
- 高性能

**Cobra**:
- CLI 菜单系统
- 命令解析
- 帮助生成

**PTY**:
- 伪终端模拟
- 真实终端体验
- 支持 ANSI 控制码

## 12. 错误处理

### 12.1 统一错误响应

```go
func handleError(c *gin.Context, err error) {
    c.JSON(http.StatusInternalServerError, gin.H{
        "success": false,
        "error":   err.Error(),
    })
}
```

### 12.2 Context 错误处理

```go
ctx, cancel := context.WithCancel(session.GetContext())
go func() {
    <-ctx.Done()
    if cmd.Process != nil {
        cmd.Process.Kill()
    }
}()
```

**优势**:
- 会话断开自动取消
- 防止僵尸进程
- 清理资源

## 13. 安全实践

### 13.1 命令注入防护

```go
// 使用固定的命令名，不接受用户输入
cmd := exec.CommandContext(ctx, ex, "--shell")

// 参数过滤
re := regexp.MustCompile(`(?m)^-?f$|^-\S+f\S*$`)
for _, str := range args {
    if len(re.FindAllString(str, -1)) != 0 {
        return []string{}, errors.New("dangerous flag")
    }
}
```

### 13.2 WebSocket 认证

```go
CheckOrigin: func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    if origin == "" {
        return true
    }
    u, err := url.Parse(origin)
    if err != nil {
        return false
    }
    return strings.EqualFold(u.Host, r.Host)
}
```

### 13.3 会话隔离

```go
v, _ := c.Get("clientSession")
clientSession := v.(*client.ClientSession)
// 每个请求独立使用自己的会话
```

## 14. 性能优化

### 14.1 Gin Release 模式

```go
gin.SetMode(gin.ReleaseMode)
```

**效果**: 
- 禁用调试日志
- 减少 CPU 使用

### 14.2 缓冲通道

```go
channel := make(chan *client.Message, 64)
```

**优势**:
- 减少阻塞
- 提高吞吐量

### 14.3 读写锁

```go
clientsMu.RLock()  // 读锁，支持并发
defer clientsMu.RUnlock()
```

**适用场景**:
- 读多写少
- 会话查询频繁

## 15. 测试

### 15.1 单元测试

```bash
# 运行所有测试
go test ./...

# 测试特定包
go test ./als/client
go test ./config

# 带覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 15.2 测试示例

**队列测试** (`backend/als/client/queue_test.go`):

```go
func TestHandleQueue(t *testing.T) {
    // 测试消息队列处理
}
```

## 16. 调试技巧

### 16.1 日志输出

```go
log.Default().Println("Loading config from environment variables...")
log.Default().Printf("Cleaned up %d expired sessions\n", removed)
```

### 16.2 调试 HTTP

```bash
# 查看详细请求
curl -v http://localhost/session

# 测试 Ping
curl "http://localhost/method/ping?target=8.8.8.8" \
  -H "Session: <session-id>"
```

### 16.3 WebSocket 调试

```bash
# 使用 wscat
wscat -c ws://localhost/session/<session-id>/shell
```

## 17. 相关文件

- [config.md](./config.md) - 配置模块详解
- [fakeshell.md](./fakeshell.md) - Fake Shell 详解
- [会话机制](../专有概念/会话机制.md) - 核心概念
