# ALS 项目文档索引

**项目名称**: ALS (Another Looking-glass Server)

**文档版本**: 1.0

**最后更新**: 2026-06-23
<!-- 同步来源: ui/ 模块代码（vite.config.js / vite.shared.js / vitest.config.js / eslint.config.js / package.json）-->

## 快速导航

### 核心文档

| 文档 | 描述 |
|------|------|
| [系统架构](./architecture.md) | 系统整体架构、技术栈和组件说明 |
| [接口文档](./interfaces.md) | API 接口规范和使用说明 |
| [开发者指南](./developer-guide.md) | 开发环境搭建、构建和测试指南 |

### 专有概念

- [会话机制](./session.md) - 客户端会话管理机制
- [功能开关](./feature-flags.md) - 基于环境变量的功能配置
- [Fake Shell](./console.md) - 限制性交互式控制台

### 模块文档

- [后端核心模块](./backend.md) - Go 后端核心逻辑
- [后端性能基线](./backend-perf.md) - benchmark 数值与优化轨迹
- [前端模块](./ui.md) - Vue.js 前端界面
- [配置模块](./config.md) - 配置加载和管理
- [FakeShell 模块](./fakeshell.md) - 限制性 shell 实现

## 项目概述

ALS 是一个轻量级的 Looking-glass 服务器，用于提供网络诊断和测速功能。

### 主要功能

- ✅ HTML5 速度测试 (LibreSpeed)
- ✅ Ping 测试 (IPv4/IPv6)
- ✅ iPerf3 带宽测试
- ✅ 实时网卡流量显示
- ✅ Speedtest.net 客户端
- ✅ 在线 Shell 控制台 (限制命令)
- ✅ NextTrace 路由跟踪

### 技术栈

**后端**:
- Go 1.26.4
- Gin Web Framework
- Gorilla WebSocket
- Cobra CLI

**前端**:
- Vue 3
- Vite
- Tailwind CSS
- Naive UI
- Vue I18n (多语言支持)
- Pinia (状态管理)
- Vitest (单元测试)

### 支持语言

- 简体中文
- English
- Русский
- Deutsch
- Español
- Français
- 日本語
- 한국어

### 部署方式

**Docker (推荐)**:
```bash
docker run -d --name looking-glass --restart always --network host ryachueng/looking-glass-server
```

**源码编译**:
- 参考 [开发者指南](./developer-guide.md)

## 资源链接

- [GitHub 仓库](https://github.com/upbeat-backbone-bose/als)
- [Docker Hub](https://hub.docker.com/r/ryachueng/looking-glass-server)
- [Issues](https://github.com/upbeat-backbone-bose/als/issues)
