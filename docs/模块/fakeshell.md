# FakeShell 模块

**模块路径**: `backend/fakeshell/`

**最后更新**: 2026-04-22

## 1. 模块概述

FakeShell 模块实现一个交互式的限制命令菜单系统，提供基于 PTY 的伪终端体验。它是 ALS Shell 功能的核心，负责命令的注册、解析和执行。

**核心职责**:
- 交互式 CLI 菜单
- 命令白名单管理
- 参数安全过滤
- PTY 启动和管理

## 2. 启动流程

### 2.1 入口点

**文件**: `backend/fakeshell/main.go`

```go
func HandleConsole() {
    // 1. 创建控制台应用
    app := console.New("example")
    app.NewlineBefore = true
    app.NewlineAfter = true

    // 2. 获取活动菜单
    menu := app.ActiveMenu()
    setupPrompt(menu)

    // 3. 添加中断处理
    menu.AddInterrupt(io.EOF, exitCtrlD)

    // 4. 设置命令菜单
    menu.SetCommands(defineMenuCommands())

    // 5. 启动控制台
    if err := app.Start(); err != nil {
        fmt.Println("console start failed:", err)
        os.Exit(1)
    }
}

func exitCtrlD(c *console.Console) {
    os.Exit(0)
}
```

**流程图**:
```
HandleConsole()
├── console.New("example")
├── app.ActiveMenu()
├── setupPrompt(menu)          # 设置提示符
├── menu.AddInterrupt(io.EOF, exitCtrlD)
├── menu.SetCommands(defineMenuCommands())
└── app.Start()                # 启动交互式菜单
```

### 2.2 独立运行

**独立模式**:
```bash
./als --shell
```

**效果**:
- 启动交互式 CLI
- 直接在终端显示
- Ctrl+D 退出

## 3. 命令菜单定义

### 3.1 核心函数

**文件**: `backend/fakeshell/menu.go`

```go
func defineMenuCommands() console.Commands {
    showedIsFirstTime := false
    
    return func() *cobra.Command {
        // 1. 创建根命令
        rootCmd := &cobra.Command{}
        
        // 2. 配置选项
        rootCmd.InitDefaultHelpCmd()
        rootCmd.CompletionOptions.DisableDefaultCmd = true
        rootCmd.DisableFlagsInUseLine = true
        
        // 3. 定义可用命令和开关映射
        features := map[string]bool{
            "ping":       config.Config.FeaturePing,
            "traceroute": config.Config.FeatureTraceroute,
            "nexttrace":  config.Config.FeatureTraceroute,
            "speedtest":  config.Config.FeatureSpeedtestDotNet,
            "mtr":        config.Config.FeatureMTR,
        }
        
        // 4. 参数过滤器
        argsFilter := map[string]func([]string) ([]string, error){
            "ping": func(args []string) ([]string, error) {
                var re = regexp.MustCompile(`(?m)^-?f$|^-\S+f\S*$`)
                for _, str := range args {
                    if len(re.FindAllString(str, -1)) != 0 {
                        return []string{}, errors.New("dangerous flag detected, stop running")
                    }
                }
                return args, nil
            },
        }
        
        // 5. 注册命令
        hasNotFound := false
        
        argsPassthough := func(args []string) ([]string, error) {
            return args, nil
        }
        
        for command, feature := range features {
            if feature {
                _, err := exec.LookPath(command)
                if err != nil {
                    if !showedIsFirstTime {
                        fmt.Println("Error: " + command + " is not installed")
                    }
                    hasNotFound = true
                    continue
                }
                
                filter, ok := argsFilter[command]
                if !ok {
                    filter = argsPassthough
                }
                
                commands.AddExecutableAsCommand(rootCmd, command, filter)
            }
        }
        
        if hasNotFound {
            showedIsFirstTime = true
        }
        
        // 6. 禁用帮助命令
        rootCmd.SetHelpCommand(&cobra.Command{
            Use:    "no-help",
            Hidden: true,
        })
        
        return rootCmd
    }
}
```

### 3.2 命令注册流程

```
defineMenuCommands()
├── 创建根命令
├── 配置 Cobra 选项
├── 定义功能开关映射
├── 定义参数过滤器
├── 检查工具是否安装
├── 注册可用命令
└── 禁用帮助命令
```

**说明**:
- 使用 Cobra 框架管理命令
- 根据配置动态注册命令
- 检查系统工具是否存在
- 参数过滤保护安全

## 4. 命令执行

### 4.1 命令注册

**文件**: `backend/fakeshell/commands/map.go`

```go
func AddExecutableAsCommand(cmd *cobra.Command, command string, 
                         argFilter func([]string) ([]string, error)) {
    
    cmdDefine := &cobra.Command{
        Use: command,
        Run: func(cmd *cobra.Command, args []string) {
            // 1. 验证命令名
            if command == "" || filepath.Base(command) != command {
                cmd.Println("invalid command")
                return
            }
            
            // 2. 参数过滤
            args, err := argFilter(args)
            if err != nil {
                cmd.Println(err)
                return
            }
            
            // 3. 执行命令
            c := exec.CommandContext(cmd.Context(), command, args...) // #nosec G204
            c.Env = os.Environ()
            c.Env = append(c.Env, "TERM=xterm-256color")
            c.Stdin = cmd.InOrStdin()
            c.Stdout = cmd.OutOrStdout()
            c.Stderr = cmd.OutOrStderr()
            
            // 4. 运行并处理结果
            if err := c.Run(); err != nil {
                cmd.Println(err)
            }
        },
        DisableFlagParsing: true,  // 原始参数传递
    }
    
    cmd.AddCommand(cmdDefine)
}
```

### 4.2 执行流程

**Cobra 命令执行**:
```
用户输入: ping 8.8.8.8 -c 4
├── Cobra 解析参数
├── 调用 Run 函数
├── 参数过滤 (argFilter)
├── exec.CommandContext 创建命令
├── 设置环境变量
├── 运行命令
├── 输出到终端
└── 错误处理
```

### 4.3 安全特性

**命令名验证**:
```go
if command == "" || filepath.Base(command) != command {
    cmd.Println("invalid command")
    return
}
```

**说明**: 
- 防止路径遍历攻击
- 仅接受命令名，不接受路径

**参数过滤**:
```go
args, err := argFilter(args)
if err != nil {
    cmd.Println(err)
    return
}
```

**执行上下文**:
```go
c := exec.CommandContext(cmd.Context(), command, args...)
```

**说明**: 
- 使用 Context 控制生命周期
- 会话断开自动终止命令

## 5. 提示符设置

**文件**: `backend/fakeshell/prompt.go`

**说明**:
- 配置命令行提示符
- 自定义显示格式

## 6. 可用命令清单

### 6.1 命令列表

| 命令 | 功能 | 配置开关 | 系统依赖 |
|------|------|---------|----------|
| `ping` | Ping 测试 | `FeaturePing` | ping |
| `traceroute` | 路由跟踪 | `FeatureTraceroute` | traceroute |
| `nexttrace` | 高级路由跟踪 | `FeatureTraceroute` | nexttrace |
| `mtr` | 诊断工具 | `FeatureMTR` | mtr |
| `speedtest` | 网速测试 | `FeatureSpeedtestDotNet` | speedtest |
| `help` | 显示帮助 | - | - |

### 6.2 系统依赖检测

**检测逻辑**:
```go
_, err := exec.LookPath(command)
if err != nil {
    fmt.Println("Error: " + command + " is not installed")
    hasNotFound = true
    continue
}
```

**说明**:
- 启动时检查工具是否安装
- 未安装的工具不注册
- 显示错误提示

## 7. 参数过滤

### 7.1 Ping 参数过滤

**实现**:
```go
argsFilter["ping"] = func(args []string) ([]string, error) {
    // 禁止 flood 模式
    var re = regexp.MustCompile(`(?m)^-f$|^-\S+f\S*$`)
    for _, str := range args {
        if len(re.FindAllString(str, -1)) != 0 {
            return []string{}, errors.New("dangerous flag detected, stop running")
        }
    }
    return args, nil
}
```

**危险参数**:
| 参数 | 危险原因 | 处理 |
|------|---------|------|
| `-f` | Flood ping (DDoS) | 禁止 |
| `-\S+f\S*` | 组合参数如 `-cf4` | 禁止 |

### 7.2 扩展过滤

**示例 - 限制 ping 间隔**:
```go
argsFilter["ping"] = func(args []string) ([]string, error) {
    // 检查间隔参数
    for i, arg := range args {
        if arg == "-i" && i+1 < len(args) {
            interval, err := strconv.ParseFloat(args[i+1], 64)
            if err == nil && interval < 0.2 {
                return []string{}, errors.New("interval too small")
            }
        }
    }
    return args, nil
}
```

## 8. 环境配置

### 8.1 环境变量

```go
c.Env = os.Environ()
c.Env = append(c.Env, "TERM=xterm-256color")
```

**说明**:
- 继承当前环境变量
- 设置终端类型为 xterm-256color
- 不暴露敏感环境变量

### 8.2 配置开关

**配置来源**:
```go
features := map[string]bool{
    "ping":       config.Config.FeaturePing,
    "traceroute": config.Config.FeatureTraceroute,
    "nexttrace":  config.Config.FeatureTraceroute,
    "speedtest":  config.Config.FeatureSpeedtestDotNet,
    "mtr":        config.Config.FeatureMTR,
}
```

**环境变量控制**:
```bash
# 禁用特定命令
docker run -d \
  -e UTILITIES_PING=false \
  -e UTILITIES_MTR=false \
  ryachueng/looking-glass-server
```

## 9. 错误处理

### 9.1 命令不存在

```go
_, err := exec.LookPath(command)
if err != nil {
    fmt.Println("Error: " + command + " is not installed")
    continue
}
```

### 9.2 参数过滤失败

```go
args, err := argFilter(args)
if err != nil {
    cmd.Println(err)
    return
}
```

### 9.3 执行错误

```go
if err := c.Run(); err != nil {
    cmd.Println(err)
}
```

## 10. 集成测试

### 10.1 独立测试

```bash
# 启动 Fake Shell
./als --shell

# 测试命令
> ping 8.8.8.8
> traceroute 1.1.1.1
> mtr google.com
```

### 10.2 参数过滤测试

```bash
# 正常 ping
> ping 8.8.8.8 -c 4

# 危险参数 (应被阻止)
> ping 8.8.8.8 -f
# 输出：dangerous flag detected, stop running
```

### 10.3 命令缺失测试

```bash
# 未安装 traceroute
> traceroute 1.1.1.1
# 可能输出：command not found
```

## 11. 性能优化

### 11.1 Context 复用

```go
ctx, cancel := context.WithCancel(session.GetContext())
defer cancel()

c := exec.CommandContext(ctx, command, args...)
```

**优势**:
- 会话断开自动终止
- 资源及时释放
- 防止僵尸进程

### 11.2 流式输出

```go
c.Stdin = cmd.InOrStdin()
c.Stdout = cmd.OutOrStdout()
c.Stderr = cmd.OutOrStderr()
```

**优势**:
- 实时输出
- 减少内存占用
- 原生体验

## 12. 安全加固建议

### 12.1 增强参数过滤

```go
argsFilter["ping"] = func(args []string) ([]string, error) {
    safeArgs := []string{}
    
    for _, arg := range args {
        // 过滤特殊字符
        if strings.ContainsAny(arg, "|;&$`\\") {
            return nil, errors.New("invalid characters in argument")
        }
        
        // 限制地址格式
        if !strings.HasPrefix(arg, "-") {
            // 验证是否为有效 IP 或域名
            if net.ParseIP(arg) == nil && !isValidDomain(arg) {
                return nil, errors.New("invalid target")
            }
        }
        
        safeArgs = append(safeArgs, arg)
    }
    
    return safeArgs, nil
}
```

### 12.2 速率限制

**在 WebSocket 层实现**:
```go
var rateLimiter = rate.NewLimiter(rate.Every(time.Second), 10)

func handleNewConnection(...) {
    // 每次命令前检查速率
    if !rateLimiter.Allow() {
        term.WriteString("Rate limit exceeded")
        return
    }
}
```

### 12.3 超时控制

```go
ctx, cancel := context.WithTimeout(session.GetContext(), 30*time.Second)
defer cancel()

c := exec.CommandContext(ctx, command, args...)
```

## 13. 调试技巧

### 13.1 日志输出

```go
fmt.Printf("Executing: %s %v\n", command, args)
```

### 13.2 独立调试

```bash
# 启动 Fake Shell
./als --shell

# 观察输出
> ping 8.8.8.8
PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=56 time=10.5 ms
```

### 13.3 系统调用跟踪

```bash
# 使用 strace 观察
strace -f ./als --shell
```

## 14. 依赖关系

### 14.1 内部依赖

| 模块 | 用途 |
|------|------|
| `backend/config` | 读取功能开关配置 |
| `backend/als/controller/shell` | 启动 Fake Shell |

### 14.2 外部依赖

| 库 | 用途 |
|------|------|
| `github.com/reeflective/console` | 交互式 CLI 框架 |
| `github.com/spf13/cobra` | 命令解析 |
| `github.com/creack/pty` | PTY 支持 |

## 15. 相关文件

- [后端核心模块](./backend.md) - 模块调用关系
- [控制台](../专有概念/控制台.md) - 详细实现和用户界面
- [会话机制](../专有概念/会话机制.md) - WebSocket 集成
