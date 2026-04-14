[![Build with release](https://github.com/upbeat-backbone-bose/als/actions/workflows/release.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/release.yml)
[![docker image build](https://github.com/upbeat-backbone-bose/als/actions/workflows/docker-image.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/docker-image.yml)
[![CI](https://github.com/upbeat-backbone-bose/als/actions/workflows/ci.yml/badge.svg)](https://github.com/upbeat-backbone-bose/als/actions/workflows/ci.yml)

Language: English | [简体中文](README_zh_CN.md)

# ALS - Another Looking-glass Server

## Supported UI Languages
- English
- Simplified Chinese
- Russian
- German
- Spanish
- French
- Japanese
- Korean

## Quick start
```bash
docker run -d --name looking-glass --restart always --network host ryachueng/looking-glass-server
```
If you don't want to use Docker, you can use the [compiled server](https://github.com/upbeat-backbone-bose/als/releases)

## Host Requirements
- RAM: 32MB or more
- Network: Host network mode required for full functionality

## How to change config
```bash
# You need to pass -e KEY=VALUE to docker command
# You can find the KEY below in the Environment Variable Table
# For example, change the listen port to 8080
docker run -d \
    --name looking-glass \
    -e HTTP_PORT=8080 \
    --restart always \
    --network host \
    ryachueng/looking-glass-server
```

## Environment Variable Table

| Key                       | Example                                                              | Default                                                    | Description                                                                             |
| ------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| LISTEN_IP                 | 127.0.0.1                                                             | (all ip)                                                   | Which IP address will be listen use                                                      |
| HTTP_PORT                 | 80                                                                    | 80                                                         | Which HTTP port should use                                                               |
| SPEEDTEST_FILE_LIST       | 100MB 1GB                                                            | 1MB 10MB 100MB 1GB                                        | Size of static test files, separate with space                                          |
| LOCATION                  | "this is location"                                                   | (request from http://ipapi.co)                             | Location string                                                                         |
| PUBLIC_IPV4               | 1.1.1.1                                                              | (fetch from http://ifconfig.co)                            | The IPv4 address of the server                                                          |
| PUBLIC_IPV6               | fe80::1                                                              | (fetch from http://ifconfig.co)                            | The IPv6 address of the server                                                          |
| DISPLAY_TRAFFIC           | true                                                                 | true                                                       | Toggle the streaming traffic graph                                                       |
| ENABLE_SPEEDTEST          | true                                                                 | true                                                       | Toggle the speedtest feature                                                            |
| UTILITIES_PING            | true                                                                 | true                                                       | Toggle the ping feature                                                                 |
| UTILITIES_SPEEDTESTDOTNET | true                                                                 | true                                                       | Toggle the speedtest.net feature                                                        |
| UTILITIES_FAKESHELL       | true                                                                 | true                                                       | Toggle the HTML Shell feature                                                           |
| UTILITIES_IPERF3          | true                                                                 | true                                                       | Toggle the iperf3 feature                                                               |
| UTILITIES_IPERF3_PORT_MIN | 30000                                                                | 30000                                                      | iperf3 listen port range - from                                                         |
| UTILITIES_IPERF3_PORT_MAX | 31000                                                               | 31000                                                      | iperf3 listen port range - to                                                           |
| SPONSOR_MESSAGE           | "Test message" or "/tmp/als_readme.md" or "http://some_host/114514.md" | ''                                                         | Show server sponsor message (support markdown file, required mapping file to container) |

## Features
- [x] HTML 5 Speed Test
- [x] Ping - IPv4 / IPv6
- [x] iPerf3 server
- [x] Streaming traffic graph
- [x] Speedtest.net Client
- [x] Online shell box (limited commands)
- [x] [NextTrace](https://github.com/nxtrace/NTrace-core) Support

## Thanks to
- [librespeed/speedtest](https://github.com/librespeed/speedtest) - Speedtest backend
- [JetBrains](https://www.jetbrains.com/) - Open source license for GoLand

## License

Code is licensed under MIT Public License.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=upbeat-backbone-bose/als&type=Date)](https://star-history.com/#upbeat-backbone-bose/als&Date)
