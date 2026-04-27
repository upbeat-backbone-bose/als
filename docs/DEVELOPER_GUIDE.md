# 开发者指南

**最后更新**: 2026-04-22

## 1. 开发环境搭建

### 1.1 依赖要求

**系统要求**:
- Linux / macOS / Windows (WSL)
- 内存：≥ 512MB
- 磁盘：≥ 1GB 可用空间

**必需软件**:

| 软件 | 版本 | 用途 |
|------|------|------|
| Go | 1.26+ | 后端开发 |
| Node.js | 22+ | 前端开发 |
| npm | 10+ | 前端依赖管理 |
| Git | 最新 | 版本控制 |
| Docker | 可选 | 容器化部署 |

**可选工具**:
- `iperf3` - 带宽测试
- `ping` - 网络诊断
- `mtr` - 路由跟踪
- `traceroute` - 路由跟踪
- `speedtest-cli` - Speedtest.net

### 1.2 克隆项目

```bash
git clone --recursive https://github.com/upbeat-backbone-bose/als.git
cd als
```

**注意**: 使用 `--recursive` 克隆子模块 (LibreSpeed)

### 1.3 安装依赖

**前端**:
```bash
cd ui
npm install
```

**后端**:
```bash
cd backend
go mod tidy
```

### 1.4 环境变量配置

创建 `.env` 文件 (可选):

```bash
# 开发环境配置
LISTEN_IP=127.0.0.1
HTTP_PORT=8080
LOCATION="Development Server"
PUBLIC_IPV4=127.0.0.1
ENABLE_SPEEDTEST=true
UTILITIES_FAKESHELL=true
UTILITIES_PING=true
UTILITIES_IPERF3=true
```

## 2. 开发与构建

### 2.1 开发模式

**前端开发服务器**:
```bash
cd ui
npm run dev
```

- 访问：`http://localhost:5173`
- 功能：热重载、HMR
- 代理：需要配置 vite.config.js 的 proxy

**后端开发**:
```bash
cd backend
go run .
```

- 监听：`http://0.0.0.0:80`
- 日志：实时输出到控制台

**同时运行前后端**:
```bash
# 终端 1 - 前端
cd ui && npm run dev

# 终端 2 - 后端
cd backend && go run .
```

### 2.2 构建生产版本

**完整构建流程**:

```bash
# 1. 构建前端
cd ui
npm run build

# 2. 复制前端到后端嵌入目录
cp -r dist ../backend/embed/ui

# 3. 构建后端
cd ../backend
go build -o als
```

**一键构建脚本**:
```bash
cd backend
go build -o als
```

**说明**: 
- 前端构建物输出到 `ui/dist/`
- 需要手动复制到 `backend/embed/ui/`
- Go embed 自动嵌入静态文件

### 2.3 交叉编译

**多平台构建**:
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o als-linux-amd64

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o als-linux-arm64

# macOS
GOOS=darwin GOARCH=amd64 go build -o als-darwin

# Windows
GOOS=windows GOARCH=amd64 go build -o als-windows.exe
```

## 3. 测试

### 3.1 单元测试

**后端测试**:
```bash
cd backend
go test ./...
```

**覆盖度报告**:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**特定包测试**:
```bash
go test ./als/client
go test ./config
```

### 3.2 前端测试

**Linting**:
```bash
cd ui
npm run lint
```

**格式化**:
```bash
npm run format
```

**构建测试**:
```bash
npm run build
```

### 3.3 集成测试

**LibreSpeed 测试**:
```bash
cd ui/speedtest
npm install
npm run test:e2e
```

## 4. Docker 开发

### 4.1 本地构建 Docker 镜像

```bash
docker build -t als:local .
```

**多架构构建**:
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t als:local \
  --load \
  .
```

### 4.2 运行开发容器

```bash
docker run -d --name als-dev \
  -p 8080:80 \
  -e HTTP_PORT=8080 \
  -e UTILITIES_FAKESHELL=true \
  als:local
```

**查看日志**:
```bash
docker logs -f als-dev
```

### 4.3 挂载开发目录

```bash
docker run -d --name als-dev \
  -p 8080:80 \
  -v $(pwd)/ui/src:/app/ui/src \
  als:local
```

**说明**: 
- 前端源码挂载实现热重载
- 需要修改 Dockerfile

## 5. 调试

### 5.1 后端调试

**启用调试日志**:
```go
// 在 main.go 中添加
log.SetFlags(log.LstdFlags | log.Lshortfile)
```

**使用 Delve**:
```bash
# 安装 dlv
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试运行
cd backend
dlv debug .
```

**HTTP 请求日志**:
```bash
# 使用 curl -v 查看详细请求
curl -v http://localhost/method/ping?target=8.8.8.8
```

### 5.2 前端调试

**开发工具**:
- Chrome DevTools - 网络、控制台
- Vue DevTools - 组件树、状态
- Network - 查看 SSE/WebSocket 连接

**调试 SSE**:
```javascript
const es = new EventSource('/session');
es.addEventListener('message', (e) => console.log('SSE:', e));
```

**调试 WebSocket**:
```javascript
const ws = new WebSocket(url);
ws.onopen = () => console.log('WebSocket connected');
ws.onmessage = (e) => console.log('Message:', e.data);
ws.onerror = (e) => console.error('Error:', e);
```

### 5.3 Shell 调试

**查看 Shell 状态**:
```bash
# 连接到 Shell
wscat -c ws://localhost/session/<session-id>/shell

# 发送 help 命令
1help
```

**假 Shell 独立测试**:
```bash
cd backend
go run . --shell
```

**启用假 Shell 调试模式**:
```bash
# 修改 fakeshell/main.go
util.SetDebug(true)  # 启用调试输出
```

## 6. 代码规范

### 6.1 Go 代码规范

**代码格式**:
```bash
gofmt -w .
go fmt ./...
```

**静态检查**:
```bash
go vet ./...
```

**安全扫描**:
```bash
# 安装 gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# 运行扫描
gosec ./...
```

**代码规范**:
- 遵循 Go 官方代码规范
- 函数名、变量名使用驼峰
- 包名全小写
- 错误处理完整
- 必要的注释

### 6.2 前端代码规范

**ESLint**:
```bash
cd ui
npm run lint
```

**Prettier**:
```bash
npm run format
```

**规范**:
- 使用 Composition API
- 组件名 PascalCase
- 文件名 PascalCase.vue
- 使用 TypeScript (推荐)
- 必要的 JSDoc 注释

## 7. 项目结构约定

### 7.1 目录结构

```
als/
├── backend/
│   ├── main.go              # 入口
│   ├── als/                 # 核心模块
│   ├   ├── route.go         # 路由配置
│   │   └── controller/      # 控制器
│   ├── config/              # 配置
│   └── fakeshell/           # Shell 实现
├── ui/
│   ├── src/
│   │   ├── components/      # Vue 组件
│   │   ├── config/          # 配置
│   │   └── locales/         # 国际化
│   └── public/
├── scripts/                 # 脚本
└── .github/                 # GitHub Actions
```

### 7.2 文件命名

**Go**:
- 文件名：小写 + 下划线
- 包名：简短、描述性

**前端**:
- 组件：PascalCase.vue (如 `Information.vue`)
- 配置：kebab-case.js (如 `lang.js`)
- 翻译：语言代码.json (如 `zh-CN.json`)

**文档**:
- Markdown：大写 + 下划线 (如 `DEVELOPER_GUIDE.md`)

### 7.3 提交规范

**Commit Message 格式**:
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type**:
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式
- `refactor`: 重构
- `test`: 测试
- `chore`: 构建/工具

**示例**:
```
feat(shell): 添加 MTR 命令支持

- 在 fakeshell/menu.go 中添加 mtr 命令
- 更新 config 添加 UTILITIES_MTR 开关

Closes #123
```

## 8. 发布流程

### 8.1 版本管理

**语义化版本**:
```
主版本。次版本.修订版本
例如：2.1.0
```

**打标签**:
```bash
git tag -a v2.1.0 -m "Release v2.1.0"
git push origin v2.1.0
```

### 8.2 发布步骤

1. **更新版本号**:
   ```bash
   # ui/package.json
   "version": "2.1.0"
   ```

2. **更新 CHANGELOG**:
   - 记录新增功能
   - 记录修复问题
   - 记录变更

3. **运行测试**:
   ```bash
   cd backend && go test ./...
   cd ui && npm run lint && npm run build
   ```

4. **打 Tag 并推送**:
   ```bash
   git tag -a v2.1.0 -m "Release v2.1.0"
   git push origin v2.1.0
   ```

5. **GitHub Actions 自动发布**:
   - 自动构建多平台二进制
   - 自动构建 Docker 镜像
   - 自动创建 GitHub Release

### 8.3 发布后检查

- [ ] GitHub Release 包含所有二进制文件
- [ ] Docker Hub 镜像已更新
- [ ] README 版本已更新
- [ ] 文档已同步更新

## 9. 故障排查

### 9.1 常见问题

**前端构建失败**:
```bash
cd ui
rm -rf node_modules package-lock.json
npm install
npm run build
```

**后端依赖冲突**:
```bash
cd backend
rm go.sum
go mod tidy
```

**嵌入文件找不到**:
```bash
# 检查 embed/ui 目录是否存在
ls -l backend/embed/ui

# 如果不存在，从 ui/dist 复制
cp -r ui/dist backend/embed/ui
```

**Docker 构建失败**:
```bash
# 清理构建缓存
docker builder prune -a

# 重新构建
docker build --no-cache -t als:local .
```

### 9.2 Debug Checklist

- [ ] Go 版本 ≥ 1.26
- [ ] Node.js 版本 ≥ 22
- [ ] 前端已构建 (`ui/dist/` 存在)
- [ ] 依赖已安装 (`go mod tidy` 已完成)
- [ ] 端口未被占用
- [ ] 环境变量正确

## 10. 性能优化

### 10.1 前端优化

**代码分割**:
```javascript
// vite.config.js
export default {
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'pinia', 'vue-i18n']
        }
      }
    }
  }
}
```

**懒加载组件**:
```javascript
import { defineAsyncComponent } from 'vue'

const ShellComponent = defineAsyncComponent(() =>
  import('./Utilities/Shell.vue')
)
```

### 10.2 后端优化

**Gin 性能调优**:
```go
gin.SetMode(gin.ReleaseMode)  # 生产环境
```

**内存优化**:
```go
// 定期清理过期会话
go func() {
  ticker := time.NewTicker(1 * time.Hour)
  for range ticker.C {
    client.RemoveExpiredClients()
  }
}()
```

**并发控制**:
```go
// 使用上下文控制
ctx, cancel := context.WithCancel(session.GetContext())
defer cancel()
```

## 11. 贡献指南

### 11.1 提交流程

1. Fork 仓库
2. 创建特性分支
3. 提交变更
4. 推送到分支
5. 创建 Pull Request

### 11.2 PR 要求

- 代码遵循项目规范
- 通过所有测试
- 必要的文档更新
- 描述清晰的变更说明

### 11.3 代码审查要点

- 功能完整性
- 代码质量
- 安全性
- 性能影响

## 12. 资源链接

- [Go 官方文档](https://go.dev/doc/)
- [Vue 3 官方文档](https://vuejs.org/)
- [Gin 官方文档](https://gin-gonic.com/)
- [LibreSpeed](https://librespeed.org/)
- [项目 GitHub](https://github.com/upbeat-backbone-bose/als)
