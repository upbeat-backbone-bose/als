[![Build with release](https://github.com/upbeat-backbone-bose/als/actions/workflows/release.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/release.yml)
[![docker image build](https://github.com/upbeat-backbone-bose/als/actions/workflows/docker-image.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/docker-image.yml)
[![CI](https://github.com/upbeat-backbone-bose/als/actions/workflows/ci.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/ci.yml)

语言: [English](README.md) | 简体中文

# ALS - 另一个 Looking-glass 服务器

## 当前支持的界面语言
- English
- 简体中文
- Русский
- Deutsch
- Español
- Français
- 日本語
- 한국어

## 快速开始 (Docker 环境)
```bash
docker run -d --name looking-glass --restart always --network host ryachueng/looking-glass-server
```
如果不想使用 Docker，您可以使用编译好的[服务器端](https://github.com/upbeat-backbone-bose/als/releases)

## 配置要求
- 内存: 32MB 或更高
- 网络: 需要主机网络模式以获得完整功能

## 如何修改配置
```bash
# 你需要在 docker 命令中传递环境变量设置参数: -e KEY=VALUE
# 你可以在下面的环境变量表中找到 KEY
# 例如，将监听端口改为 8080
docker run -d \
    --name looking-glass \
    -e HTTP_PORT=8080 \
    --restart always \
    --network host \
    ryachueng/looking-glass-server
```

## 环境变量表

| Key                       | 示例                                                                 | 默认                                                       | 描述                                                                                     |
| ------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| LISTEN_IP                 | 127.0.0.1                                                            | (全部 IP)                                                  | 监听在哪一个 IP 上                                                                       |
| HTTP_PORT                 | 80                                                                   | 80                                                         | 监听在哪一个端口上                                                                       |
| SPEEDTEST_FILE_LIST       | 100MB 1GB                                                            | 1MB 10MB 100MB 1GB                                         | 静态文件大小列表，使用空格隔开                                                           |
| LOCATION                  | "this is location"                                                  | (请求 ipapi.co 获取)                                       | 服务器位置的文本                                                                         |
| PUBLIC_IPV4               | 1.1.1.1                                                              | (从在线获取)                                               | 服务器的 IPv4 地址                                                                       |
| PUBLIC_IPV6               | fe80::1                                                              | (从在线获取)                                               | 服务器的 IPv6 地址                                                                       |
| DISPLAY_TRAFFIC           | true                                                                 | true                                                       | 实时流量开关                                                                             |
| ENABLE_SPEEDTEST          | true                                                                 | true                                                       | 测速功能开关                                                                             |
| UTILITIES_PING            | true                                                                 | true                                                       | Ping 功能开关                                                                            |
| UTILITIES_SPEEDTESTDOTNET | true                                                                | true                                                       | Speedtest.net 功能开关                                                                  |
| UTILITIES_FAKESHELL       | true                                                                 | true                                                       | Shell 功能开关                                                                           |
| UTILITIES_IPERF3          | true                                                                 | true                                                       | iPerf3 服务器功能开关                                                                   |
| UTILITIES_IPERF3_PORT_MIN | 30000                                                                | 30000                                                      | iPerf3 服务器端口范围 - 开始                                                             |
| UTILITIES_IPERF3_PORT_MAX | 31000                                                                | 31000                                                      | iPerf3 服务器端口范围 - 结束                                                             |
| SPONSOR_MESSAGE           | "Test message" or "/tmp/als_readme.md" or "http://some_host/114514.md" | ''                                                         | 显示节点赞助商信息 (支持 Markdown, 支持 URL/文字/文件 (文件需要映射到容器中，使用映射后的路径) |

## 功能
- [x] HTML 5 速度测试
- [x] Ping - IPv4 / IPv6
- [x] iPerf3 服务器控制
- [x] 实时网卡流量显示
- [x] Speedtest.net 客户端
- [x] 在线 shell 盒子 (限制命令)
- [x] [NextTrace](https://github.com/nxtrace/NTrace-core) 支持

## 致谢
- [librespeed/speedtest](https://github.com/librespeed/speedtest) - 测速后端
- [JetBrains](https://www.jetbrains.com/) - GoLand 开源许可

## 许可证

代码采用 MIT 公共许可证授权。

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=upbeat-backbone-bose/als&type=Date)](https://star-history.com/#upbeat-backbone-bose/als&Date)
