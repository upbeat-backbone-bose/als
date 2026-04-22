# 接口文档

**最后更新**: 2026-04-22

## 1. 概述

本文档描述 ALS 系统的所有 HTTP 和 WebSocket 接口。

### 1.1 基础信息

**Base URL**: `http(s)://<host>:<port>`

**默认端口**: 80

### 1.2 认证机制

系统使用**基于会话的认证**:

1. 客户端调用 `GET /session` 创建会话
2. 服务器通过 SSE 推送会话 ID (UUID)
3. 后续请求携带会话 ID：
   - HTTP Header: `Session: <session-id>`
   - URL 参数：`/session/<session-id>/...`

**会话有效期**: 24 小时

### 1.3 错误响应格式

```json
{
  "success": false,
  "error": "错误描述"
}
```

## 2. HTTP API

### 2.1 会话管理

#### `GET /session`

创建新会话，通过 SSE 推送会话 ID 和配置。

**认证**: 无

**响应内容类型**: `text/event-stream`

**SSE 事件**:

**事件 1 - SessionId**:
```
event: SessionId
data: <uuid-string>
```

**事件 2 - Config**:
```
event: Config
data: {
  "location": "服务器位置",
  "public_ipv4": "1.2.3.4",
  "public_ipv6": "2001:db8::1",
  "feature_ping": true,
  "feature_shell": true,
  "feature_librespeed": true,
  "feature_filespeedtest": true,
  "feature_speedtest_dot_net": true,
  "feature_iperf3": true,
  "feature_mtr": true,
  "feature_traceroute": true,
  "feature_iface_traffic": true,
  "speedtest_files": ["1MB", "10MB", "100MB", "1GB"],
  "sponsor_message": "赞助商信息",
  "my_ip": "客户端 IP"
}
```

**事件 3 - InterfaceCache**:
```
event: InterfaceCache
data: [
  {
    "name": "eth0",
    "speed": "10Gbps",
    "mac": "00:11:22:33:44:55"
  }
]
```

**后续事件**:
- `InterfaceTraffic`: 实时网卡流量 (来自定时广播)
- `SystemResource`: 系统资源使用 (来自定时广播)

**客户端示例**:

```javascript
const eventSource = new EventSource('/session');

eventSource.addEventListener('SessionId', (event) => {
  const sessionId = event.data;
  localStorage.setItem('sessionId', sessionId);
  
  // 后续请求携带 sessionId
  // 方式 1: Header
  // fetch('/method/ping', { headers: { 'Session': sessionId } })
  
  // 方式 2: URL
  fetch(`/session/${sessionId}/shell`)
});

eventSource.addEventListener('Config', (event) => {
  const config = JSON.parse(event.data);
  // 根据 feature_* 字段启用/禁用 UI 功能
});
```

---

### 2.2 网络工具 (使用 Session Header)

#### `GET /method/ping`

执行 Ping 测试。

**认证**: `Session` Header

**请求参数**:
| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `target` | string | 是 | 目标地址 (域名或 IP) |
| `count` | int | 否 | Ping 次数 (默认 4) |
| `interval` | int | 否 | 间隔毫秒 (默认 1000) |

**响应**:
```
正在执行 ping...
PING 目标地址...
结果输出...
```

**示例**:
```bash
curl -H "Session: <session-id>" \
  "http://localhost/method/ping?target=8.8.8.8&count=4"
```

---

#### `GET /method/iperf3/server`

获取 iPerf3 服务器连接信息。

**认证**: `Session` Header

**响应**:
```json
{
  "server": "1.2.3.4",
  "port": 30001
}
```

**说明**: 
- 端口在配置范围内动态分配
- 范围：`UTILITIES_IPERF3_PORT_MIN` 到 `UTILITIES_IPERF3_PORT_MAX`

**客户端使用**:
```javascript
fetch('/method/iperf3/server', {
  headers: { 'Session': sessionId }
})
.then(res => res.json())
.then(data => {
  // 连接 iPerf3: iperf3 -c data.server -p data.port
});
```

---

#### `GET /method/speedtest_dot_net`

执行 Speedtest.net CLI 测试。

**认证**: `Session` Header

**响应**: 文本输出 (speedtest-cli 工具的输出)

**说明**: 需要服务器安装 `speedtest-cli` 工具

---

#### `GET /method/cache/interfaces`

获取网卡接口缓存信息。

**认证**: `Session` Header

**响应**:
```json
[
  {
    "name": "eth0",
    "speed": "10Gbps",
    "mac": "00:11:22:33:44:55",
    "ip": "192.168.1.100",
    "rx_bytes": 123456789,
    "tx_bytes": 987654321
  }
]
```

**说明**: 
- 数据来自 `/sys/class/net/`
- 定时更新 (1 秒间隔)

---

### 2.3 会话相关接口 (使用 URL 参数)

#### `GET /session/:session/shell`

建立 WebSocket Shell 连接。

**认证**: URL 参数 `:session`

**协议**: WebSocket

**升级响应**: `101 Switching Protocols`

**消息格式**:

**客户端 → 服务器** (二进制消息):
```
1<input-data>           # 输入文本
2<height>;<width>        # 调整窗口大小
```

**服务器 → 客户端** (二进制消息):
```
<output-data>           # Shell 输出
```

**JavaScript 示例**:

```javascript
const ws = new WebSocket(`ws://localhost/session/${sessionId}/shell`);

ws.onopen = () => {
  // 发送命令
  const encoder = new TextEncoder();
  ws.send(encoder.encode('1ls -la\n'));
  
  // 调整窗口
  ws.send(encoder.encode('224;80'));
};

ws.onmessage = (event) => {
  const decoder = new TextDecoder();
  const output = decoder.decode(event.data);
  // 渲染到终端
};
```

**可用命令** (可配置):
- `ping` - Ping 测试
- `traceroute` / `nexttrace` - 路由跟踪
- `mtr` - MTR 诊断
- `speedtest` - Speedtest 测试
- `help` - 显示帮助

**安全限制**:
- 命令白名单
- 危险参数过滤
- 超时自动断开

---

#### `GET /session/:session/speedtest/file/:filename`

下载测速文件。

**认证**: URL 参数 `:session`

**路径参数**:
- `filename`: 文件名 (如 "100MB", "1GB")

**响应**: 
- Content-Type: `application/octet-stream`
- Content-Length: 文件大小

**说明**: 
- 文件大小从 `SPEEDTEST_FILE_LIST` 配置
- 动态生成 (全零数据)

---

#### `GET /session/:session/speedtest/download`

LibreSpeed 下载测试端点。

**认证**: URL 参数 `:session`

**查询参数**:
| 参数 | 类型 | 描述 |
|------|------|------|
| `size` | int | 数据块大小 (MB) |

**响应**: 随机二进制数据流

**说明**: 
- 使用 `Math.random()` 生成随机数据
- 用于模拟真实下载

---

#### `POST /session/:session/speedtest/upload`

LibreSpeed 上传测试端点。

**认证**: URL 参数 `:session`

**请求体**: 二进制数据

**响应**: 空响应 (仅验证接收)

**说明**: 
- 接收数据但不存储
- 通过上传时间计算速度

---

### 2.4 静态资源

#### `GET /`

返回前端单页应用入口。

**响应**: HTML

**嵌入**: `embed/ui/index.html`

---

#### `GET /assets/:filename`

返回前端静态资源。

**路径参数**:
- `filename`: 文件名 (如 `main.js`, `style.css`)

**响应**: 
- JavaScript 文件: `application/javascript`
- CSS 文件: `text/css`
- 图片：对应 MIME 类型

**嵌入**: `embed/ui/assets/`

---

#### `GET /speedtest_worker.js`

返回 LibreSpeed Web Worker。

**响应**: JavaScript

**嵌入**: `public/speedtest_worker.js`

---

#### `GET /favicon.ico`

返回网站图标。

**响应**: 图像/x-icon

**嵌入**: `embed/ui/favicon.ico`

---

## 3. WebSocket 消息协议

### 3.1 Shell 连接

**连接 URL**: `ws(s)://host/session/<session-id>/shell`

**握手要求**:
- Header: `Upgrade: websocket`
- Header: `Connection: Upgrade`
- Header: `Sec-WebSocket-Key: <base64>`
- Header: `Sec-WebSocket-Protocol: binary`

**连接建立后**:

| 方向 | 格式 | 描述 |
|------|------|------|
| 客户端 → 服务器 | `1<text>` | 发送输入 (如 "ls\n") |
| 客户端 → 服务器 | `2<rows>;<cols>` | 调整终端大小 |
| 服务器 → 客户端 | `<text>` | Shell 输出 |

**完整示例**:

```javascript
const ws = new WebSocket('ws://localhost/session/uuid/shell');

ws.binaryType = 'arraybuffer';

// 发送命令
function sendCommand(cmd) {
  const data = '1' + cmd + '\n';
  ws.send(new TextEncoder().encode(data));
}

// 调整窗口
function resize(rows, cols) {
  const data = `2${rows};${cols}`;
  ws.send(new TextEncoder().encode(data));
}

// 接收输出
ws.onmessage = (event) => {
  const text = new TextDecoder().decode(event.data);
  console.log('Shell output:', text);
};
```

---

## 4. SSE 事件流

### 4.1 事件类型

#### SessionId (一次性)
```javascript
eventSource.addEventListener('SessionId', (e) => {
  console.log('Session ID:', e.data);
});
```

#### Config (一次性)
```javascript
eventSource.addEventListener('Config', (e) => {
  const config = JSON.parse(e.data);
  console.log('Features:', config.feature_ping);
});
```

#### InterfaceCache (一次性)
```javascript
eventSource.addEventListener('InterfaceCache', (e) => {
  const interfaces = JSON.parse(e.data);
  console.log('Interfaces:', interfaces);
});
```

#### InterfaceTraffic (周期性)
```javascript
eventSource.addEventListener('InterfaceTraffic', (e) => {
  const traffic = JSON.parse(e.data);
  // 更新流量图表
});
```

#### SystemResource (周期性)
```javascript
eventSource.addEventListener('SystemResource', (e) => {
  const resource = JSON.parse(e.data);
  console.log('Memory:', resource.memoryUsage);
});
```

---

## 5. 错误处理

### 5.1 常见错误码

| HTTP 状态码 | 含义 | 场景 |
|-------------|------|------|
| 400 | 无效请求 | 会话 ID 无效、参数错误 |
| 404 | 资源不存在 | 文件测速文件名错误 |
| 500 | 服务器错误 | 命令执行失败 |

### 5.2 错误响应示例

**会话无效**:
```json
{
  "success": false,
  "error": "Invalid session"
}
```

**参数错误**:
```json
{
  "success": false,
  "error": "Missing target parameter"
}
```

**功能未启用**:
```http
HTTP/1.1 404 Not Found
Content-Type: text/plain

Not found
```

---

## 6. 速率限制

### 当前实现
- **无**: 系统当前未实现速率限制

### 建议实施
```
- Ping 测试：每分钟最多 10 次
- iPerf3：每小时最多 5 次
- Shell: 每会话最多 10 个并发命令
```

---

## 7. CORS 配置

### WebSocket
```javascript
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

**策略**: 
- 允许空 Origin
- 允许同源请求
- 拒绝跨源请求

### HTTP API
- **无明确 CORS 头**: 默认允许同源
- 前端通过反向代理访问 (无跨域问题)

---

## 8. API 使用最佳实践

### 8.1 会话管理
```javascript
// 获取会话 ID
async function getSessionId() {
  return new Promise((resolve) => {
    const es = new EventSource('/session');
    es.addEventListener('SessionId', (e) => {
      es.close();
      resolve(e.data);
    });
  });
}

// 使用
const sessionId = await getSessionId();
```

### 8.2 错误重试
```javascript
async function fetchWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const res = await fetch(url, options);
      if (res.ok) return res;
      if (res.status === 400) {
        // 会话可能过期，重新获取
        options.headers.Session = await getSessionId();
        continue;
      }
      throw new Error(`HTTP ${res.status}`);
    } catch (err) {
      if (i === maxRetries - 1) throw err;
      await new Promise(r => setTimeout(r, 1000 * (i + 1)));
    }
  }
}
```

### 8.3 资源清理
```javascript
const eventSource = new EventSource('/session');

// 组件卸载时清理
onUnmounted(() => {
  eventSource.close();
});
```

---

## 9. 测试工具

### cURL 测试

**获取会话 ID**:
```bash
curl -N http://localhost/session 2>&1 | grep SessionId | awk '{print $2}'
```

**执行 Ping**:
```bash
SESSION=$(curl -N http://localhost/session 2>&1 | grep SessionId | awk '{print $2}')
curl -H "Session: $SESSION" "http://localhost/method/ping?target=8.8.8.8"
```

**连接 Shell** (需要 wscat):
```bash
wscat -c ws://localhost/session/$SESSION/shell
```

### JavaScript 测试
```javascript
// 完整测试流程
const sessionId = await getSessionId();

// Ping
const pingRes = await fetch('/method/ping?target=8.8.8.8', {
  headers: { 'Session': sessionId }
});
console.log(await pingRes.text());

// iPerf3
const iperfRes = await fetch('/method/iperf3/server', {
  headers: { 'Session': sessionId }
});
console.log(await iperfRes.json());
```
