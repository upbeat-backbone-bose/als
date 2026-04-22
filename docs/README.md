# ALS 项目文档

**Another Looking-glass Server**

---

## 快速开始

### 文档导览

| 文档类型 | 说明 |
|---------|------|
| [系统架构](./ARCHITECTURE.md) | 系统整体架构、技术栈和组件说明 |
| [接口文档](./INTERFACES.md) | API 接口规范和使用说明 |
| [开发者指南](./DEVELOPER_GUIDE.md) | 开发环境搭建、构建和测试指南 |

### 核心概念

- [会话机制](./专有概念/会话机制.md) - 客户端会话管理机制
- [功能开关](./专有概念/功能开关.md) - 基于环境变量的功能配置
- [控制台](./专有概念/控制台.md) - 限制性交互式 Shell

### 模块文档

- [后端核心](./模块/backend.md) - Go 后端核心逻辑
- [前端模块](./模块/ui.md) - Vue.js 前端界面
- [配置模块](./模块/config.md) - 配置加载和管理
- [FakeShell](./模块/fakeshell.md) - 限制性 shell 实现

---

## 项目简介

ALS (Another Looking-glass Server) 是一个轻量级的 Looking-glass 服务器，用于提供网络诊断和测速功能。

### 主要功能

- ✅ HTML5 速度测试 (LibreSpeed)
- ✅ Ping 测试 (IPv4/IPv6)
- ✅ iPerf3 带宽测试
- ✅ 实时网卡流量显示
- ✅ Speedtest.net 客户端
- ✅ 在线 Shell 控制台 (限制命令)
- ✅ NextTrace 路由跟踪

### 快速部署

```bash
# Docker 部署
docker run -d --name looking-glass --restart always --network host ryachueng/looking-glass-server
```

### 支持语言

- 简体中文 | English | Русский | Deutsch | Español | Français | 日本語 | 한국어

---

## 文档链接

- [GitHub 仓库](https://github.com/upbeat-backbone-bose/als)
- [Docker Hub](https://hub.docker.com/r/ryachueng/looking-glass-server)
- [Issues](https://github.com/upbeat-backbone-bose/als/issues)

---

*文档最后更新：2026-04-22*
