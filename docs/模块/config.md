# 配置模块

**模块路径**: `backend/config/`

**最后更新**: 2026-04-22

## 1. 模块概述

配置模块负责 ALS 系统的配置加载、环境变量解析和默认值管理。配置在应用启动时一次性加载，运行时不可动态修改。

**核心职责**:
- 加载默认配置
- 解析环境变量
- 获取地理位置信息
- 加载赞助商消息

## 2. 配置结构

### 2.1 ALSConfig 类型

**文件**: `backend/config/init.go`

```go
type ALSConfig struct {
    // 网络配置
    ListenHost string `json:"-"`
    ListenPort string `json:"-"`

    // 服务器物理信息
    Location string `json:"location"`
    PublicIPv4 string `json:"public_ipv4"`
    PublicIPv6 string `json:"public_ipv6"`

    // iPerf3 配置
    Iperf3StartPort int `json:"-"`
    Iperf3EndPort   int `json:"-"`

    // 测速配置
    SpeedtestFileList []string `json:"speedtest_files"`

    // 赞助商信息
    SponsorMessage string `json:"sponsor_message"`

    // 功能开关
    FeaturePing            bool `json:"feature_ping"`
    FeatureShell           bool `json:"feature_shell"`
    FeatureLibrespeed      bool `json:"feature_librespeed"`
    FeatureFileSpeedtest   bool `json:"feature_filespeedtest"`
    FeatureSpeedtestDotNet bool `json:"feature_speedtest_dot_net"`
    FeatureIperf3          bool `json:"feature_iperf3"`
    FeatureMTR             bool `json:"feature_mtr"`
    FeatureTraceroute      bool `json:"feature_traceroute"`
    FeatureIfaceTraffic    bool `json:"feature_iface_traffic"`
}
```

**说明**:
- `json:"-"` 标签表示不序列化为 JSON
- 前端仅需要功能开关和服务器信息

### 2.2 全局变量

```go
var Config *ALSConfig
var IsInternalCall bool
```

**说明**:
- `Config`: 全局配置实例
- `IsInternalCall`: 标记是否为内部调用 (--shell 模式)

## 3. 配置加载流程

### 3.1 主流程

```go
func LoadWebConfig() {
    // 1. 加载默认配置 + 环境变量
    Config = GetDefaultConfig()
    LoadFromEnv()
    
    // 2. 加载赞助商消息
    LoadSponsorMessage()
    
    // 3. 输出日志
    log.Default().Println("Loading config for web services...")

    // 4. 检查 iperf3 是否安装
    _, err := exec.LookPath("iperf3")
    if err != nil {
        log.Default().Println("WARN: Disable iperf3 due to not found")
        Config.FeatureIperf3 = false
    }

    // 5. 获取公网 IP
    if Config.PublicIPv4 == "" && Config.PublicIPv6 == "" {
        go func() {
            updatePublicIP()
            if Config.Location == "" {
                updateLocation()
            }
        }()
    }
}
```

**流程图**:
```
LoadWebConfig()
├── Load()                    # 默认配置 + 环境变量
│   ├── GetDefaultConfig()    # 设置默认值
│   └── LoadFromEnv()         # 解析环境变量
├── LoadSponsorMessage()      # 加载赞助商消息
├── 检查 iperf3 是否存在
└── 获取公网 IP 和位置信息
```

### 3.2 默认配置

**文件**: `backend/config/init.go`

```go
func GetDefaultConfig() *ALSConfig {
    defaultConfig := &ALSConfig{
        ListenHost:      "0.0.0.0",
        ListenPort:      "80",
        Location:        "",
        Iperf3StartPort: 30000,
        Iperf3EndPort:   31000,

        SpeedtestFileList: []string{"1MB", "10MB", "100MB", "1GB"},
        PublicIPv4:        "",
        PublicIPv6:        "",

        // 所有功能默认启用
        FeaturePing:            true,
        FeatureShell:           true,
        FeatureLibrespeed:      true,
        FeatureFileSpeedtest:   true,
        FeatureSpeedtestDotNet: true,
        FeatureIperf3:          true,
        FeatureMTR:             true,
        FeatureTraceroute:      true,
        FeatureIfaceTraffic:    true,
    }

    return defaultConfig
}
```

**特点**:
- 监听所有接口
- 默认端口 80
- 所有功能默认启用
- 测速文件大小列表

## 4. 环境变量加载

### 4.1 实现

**文件**: `backend/config/load_from_env.go`

```go
func LoadFromEnv() {
    // 字符串类型环境变量
    envVarsString := map[string]*string{
        "LISTEN_IP":       &Config.ListenHost,
        "HTTP_PORT":       &Config.ListenPort,
        "LOCATION":        &Config.Location,
        "PUBLIC_IPV4":     &Config.PublicIPv4,
        "PUBLIC_IPV6":     &Config.PublicIPv6,
        "SPONSOR_MESSAGE": &Config.SponsorMessage,
    }

    // 整数类型
    envVarsInt := map[string]*int{
        "UTILITIES_IPERF3_PORT_MIN": &Config.Iperf3StartPort,
        "UTILITIES_IPERF3_PORT_MAX": &Config.Iperf3EndPort,
    }

    // 布尔类型
    envVarsBool := map[string]*bool{
        "DISPLAY_TRAFFIC":           &Config.FeatureIfaceTraffic,
        "ENABLE_SPEEDTEST":          &Config.FeatureLibrespeed,
        "UTILITIES_SPEEDTESTDOTNET": &Config.FeatureSpeedtestDotNet,
        "UTILITIES_PING":            &Config.FeaturePing,
        "UTILITIES_FAKESHELL":       &Config.FeatureShell,
        "UTILITIES_IPERF3":          &Config.FeatureIperf3,
        "UTILITIES_MTR":             &Config.FeatureMTR,
    }

    // 解析字符串
    for envVar, configField := range envVarsString {
        if v := os.Getenv(envVar); len(v) != 0 {
            *configField = v
        }
    }

    // 解析整数
    for envVar, configField := range envVarsInt {
        if v := os.Getenv(envVar); len(v) != 0 {
            v, err := strconv.Atoi(v)
            if err != nil {
                continue
            }
            *configField = v
        }
    }

    // 解析布尔值
    for envVar, configField := range envVarsBool {
        if v := os.Getenv(envVar); len(v) != 0 {
            *configField = v == "true"
        }
    }

    // 特殊处理：测速文件列表 (空格分隔)
    if v := os.Getenv("SPEEDTEST_FILE_LIST"); len(v) != 0 {
        fileLists := strings.Split(v, " ")
        Config.SpeedtestFileList = fileLists
    }

    // 日志
    if !IsInternalCall {
        log.Default().Println("Loading config from environment variables...")
    }
}
```

### 4.2 环境变量完整列表

| 变量名 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `LISTEN_IP` | string | "0.0.0.0" | 监听 IP 地址 |
| `HTTP_PORT` | string | "80" | 监听端口 |
| `LOCATION` | string | (自动获取) | 服务器位置描述 |
| `PUBLIC_IPV4` | string | (自动获取) | 公网 IPv4 地址 |
| `PUBLIC_IPV6` | string | (自动获取) | 公网 IPv6 地址 |
| `SPEEDTEST_FILE_LIST` | string | "1MB 10MB 100MB 1GB" | 测速文件大小列表 |
| `SPONSOR_MESSAGE` | string | "" | 赞助商消息 |
| `DISPLAY_TRAFFIC` | bool | true | 网卡流量显示开关 |
| `ENABLE_SPEEDTEST` | bool | true | LibreSpeed 开关 |
| `UTILITIES_SPEEDTESTDOTNET` | bool | true | Speedtest.net 开关 |
| `UTILITIES_PING` | bool | true | Ping 功能开关 |
| `UTILITIES_FAKESHELL` | bool | true | Shell 功能开关 |
| `UTILITIES_IPERF3` | bool | true | iPerf3 开关 |
| `UTILITIES_IPERF3_PORT_MIN` | int | 30000 | iPerf3 起始端口 |
| `UTILITIES_IPERF3_PORT_MAX` | int | 31000 | iPerf3 结束端口 |
| `UTILITIES_MTR` | bool | true | MTR 开关 |
| `UTILITIES_TRACEROUTE` | bool | true | Traceroute 开关 |

**注意**: 
- `SPEEDTEST_FILE_LIST` 使用空格分隔多个值
- 布尔值必须为 "true" 或 "false"
- 数字值使用字符串表示，自动转换

## 5. 地理位置获取

### 5.1 获取公网 IP

**文件**: `backend/config/ip.go`

```go
func updatePublicIP() {
    httpClient := &http.Client{Timeout: 5 * time.Second}

    // 获取 IPv4
    resp, err := httpClient.Get("https://api.ipify.org")
    if err == nil && resp.StatusCode == http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        Config.PublicIPv4 = string(body)
    }

    // 获取 IPv6
    resp, err = httpClient.Get("https://api6.ipify.org")
    if err == nil && resp.StatusCode == http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        Config.PublicIPv6 = string(body)
    }
}
```

**数据源**:
- IPv4: `https://api.ipify.org`
- IPv6: `https://api6.ipify.org`

### 5.2 获取地理位置

**文件**: `backend/config/location.go`

```go
func updateLocation() {
    httpClient := &http.Client{Timeout: 5 * time.Second}

    // 使用 ipapi.co
    resp, err := httpClient.Get("https://ipapi.co/json/")
    if err != nil || resp.StatusCode != http.StatusOK {
        return
    }

    var data struct {
        City     string `json:"city"`
        Region   string `json:"region"`
        Country  string `json:"country_name"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
        Config.Location = fmt.Sprintf("%s, %s, %s",
            data.City, data.Region, data.Country)
    }
}
```

**数据源**:
- `https://ipapi.co/json/`

**响应示例**:
```json
{
  "city": "San Francisco",
  "region": "California",
  "country_name": "United States"
}
```

## 6. 赞助商消息加载

**文件**: `backend/config/init.go`

```go
func LoadSponsorMessage() {
    if Config.SponsorMessage == "" {
        return
    }

    log.Default().Println("Loading sponsor message...")

    // 尝试作为文件路径读取
    if _, err := os.Stat(Config.SponsorMessage); err == nil {
        content, err := os.ReadFile(Config.SponsorMessage)
        if err == nil {
            Config.SponsorMessage = string(content)
            return
        }
    }

    // 尝试作为 URL 读取
    httpClient := &http.Client{Timeout: 5 * time.Second}
    resp, err := httpClient.Get(Config.SponsorMessage)
    if err == nil && resp.StatusCode == http.StatusOK {
        defer resp.Body.Close()
        content, _ := io.ReadAll(resp.Body)
        log.Default().Println("Loaded sponsor message from url.")
        Config.SponsorMessage = string(content)
        return
    }

    // 失败时使用原始字符串
    log.Default().Println("ERROR: Failed to load sponsor message.")
}
```

**支持三种格式**:
1. **纯文本**: 直接显示
2. **文件路径**: 读取文件内容 (支持 Markdown)
3. **URL**: 从 HTTP 端点获取 (支持 Markdown)

**使用示例**:

```bash
# 纯文本
SPONSOR_MESSAGE="感谢 XYZ 公司赞助"

# 文件路径
SPONSOR_MESSAGE="/app/sponsor.md"

# URL
SPONSOR_MESSAGE="https://example.com/sponsor.md"
```

## 7. 使用模式

### 7.1 启动时加载配置

**文件**: `backend/main.go`

```go
func main() {
    flag.Parse()
    
    if *shell {
        config.IsInternalCall = true
        config.Load()
        fakeshell.HandleConsole()
        return
    }

    config.LoadWebConfig()
    als.Init()
}
```

### 7.2 访问配置

**任何模块中**:

```go
import "github.com/samlm0/als/v2/config"

func someFunction() {
    // 读取配置
    if config.Config.FeaturePing {
        // 启用 ping 功能
    }
    
    // 访问服务器信息
    location := config.Config.Location
    publicIP := config.Config.PublicIPv4
}
```

### 7.3 路由注册

**文件**: `backend/als/route.go`

```go
func SetupHttpRoute(e *gin.Engine) {
    // 条件注册：只有 FeatureIperf3 为 true 时才注册
    if config.Config.FeatureIperf3 {
        v1.GET("/iperf3/server", iperf3.Handle)
    }
    
    if config.Config.FeaturePing {
        v1.GET("/ping", ping.Handle)
    }
}
```

## 8. 配置验证

### 8.1 启动日志

```bash
$ ./als

Loading config from environment variables...
Loading config for web services...
Listen on: 0.0.0.0:80
```

**警告示例**:
```
WARN: Disable iperf3 due to not found
```

### 8.2 前端验证

前端通过 SSE 获取配置：

```javascript
eventSource.addEventListener('Config', (e) => {
    const config = JSON.parse(e.data);
    console.log('Ping enabled:', config.feature_ping);
    console.log('Location:', config.location);
});
```

## 9. 配置优先级

```
1. GetDefaultConfig()       (最低优先级 - 默认值)
2. LoadFromEnv()            (中优先级 - 环境变量)
3. 自动检测 (iperf3)       (覆盖布尔值)
4. 自动获取 (IP/Location)   (如果未手动设置)
```

**说明**:
- 环境变量的值会覆盖默认值
- 自动检测可能会禁用某些功能
- 如果手动设置了 IP/Location，自动获取会跳过

## 10. 错误处理

### 10.1 数字转换错误

```go
if v := os.Getenv(envVar); len(v) != 0 {
    v, err := strconv.Atoi(v)
    if err != nil {
        continue  // 忽略无效数字，保留默认值
    }
    *configField = v
}
```

### 10.2 网络超时

```go
httpClient := &http.Client{Timeout: 5 * time.Second}
resp, err := httpClient.Get("https://api.ipify.org")
if err != nil {
    // 失败时不设置，保留空值
}
```

### 10.3 文件读取失败

```go
content, err := os.ReadFile(Config.SponsorMessage)
if err != nil {
    // 回退到 URL 或字符串
}
```

## 11. 最佳实践

### 11.1 环境变量命名

- 使用大写字母和下划线
- 前缀区分功能类型：
  - `UTILITIES_*` - 工具开关
  - `DISPLAY_*` - 显示相关
  - `ENABLE_*` - 启用/禁用

### 11.2 敏感配置

**不要硬编码敏感信息**:

```go
// 错误示例
Config.APIKey = "sk-12345"

// 正确示例：使用环境变量
Config.APIKey = os.Getenv("API_KEY")
```

### 11.3 配置文档化

在 README 或文档中列出所有环境变量：

```markdown
## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `HTTP_PORT` | 监听端口 | 80 |
```

## 12. 故障排查

### 12.1 配置不生效

**检查步骤**:
1. 确认环境变量名称正确
2. 检查拼写错误
3. 确认 Docker 传递了环境变量
4. 查看启动日志

**常见问题**:
```bash
# 错误：拼写错误
-e UTILITES_PING=false  # 应为 UTILITIES_

# 错误：布尔值格式
-e UTILITIES_PING=0     # 应为 "true" 或 "false"
```

### 12.2 port 范围无效

```bash
# 错误：起始端口 > 结束端口
-e UTILITIES_IPERF3_PORT_MIN=31000
-e UTILITIES_IPERF3_PORT_MAX=30000
```

**解决**: 确保 `PORT_MIN < PORT_MAX`

### 12.3 地理位置获取失败

**原因**:
- 网络不可达
- API 服务不可用
- 超时

**解决**:
- 手动设置 `LOCATION` 环境变量
- 检查网络连通性

## 13. 扩展建议

### 13.1 添加新配置

**步骤**:

1. 在 `ALSConfig` 中添加字段:
```go
type ALSConfig struct {
    // ...
    NewFeature bool `json:"new_feature"`
}
```

2. 设置默认值:
```go
func GetDefaultConfig() *ALSConfig {
    return &ALSConfig{
        // ...
        NewFeature: true,
    }
}
```

3. 添加环境变量映射:
```go
envVarsBool["ENABLE_NEW_FEATURE"] = &Config.NewFeature
```

4. 在代码中使用:
```go
if config.Config.NewFeature {
    // 新功能逻辑
}
```

### 13.2 配置验证加强

```go
func LoadWebConfig() {
    // 验证端口范围
    if Config.Iperf3StartPort >= Config.Iperf3EndPort {
        log.Fatal("Iperf3 port range invalid")
    }
    
    // 验证监听端口
    if port, _ := strconv.Atoi(Config.ListenPort); port < 1 || port > 65535 {
        log.Fatal("Invalid listen port")
    }
}
```

### 13.3 动态重载

**当前**: 配置启动时加载，不可动态修改

**建议方案**:
- 使用配置文件 + `inotify` 监听
- 实现配置热重载 API
- 使用配置服务器

## 14. 相关文件

- [功能开关](../专有概念/功能开关.md) - 配置在功能开关中的应用
- [会话机制](../专有概念/会话机制.md) - 配置通过会话推送给前端
- [后端核心模块](./backend.md) - 配置模块的调用关系
