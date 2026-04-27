# 前端模块

**模块路径**: `ui/`

**最后更新**: 2026-04-22

## 1. 模块概述

ALS 前端是一个基于 Vue 3 的现代化单页应用 (SPA)，提供网络诊断和测速功能的可视化界面。采用组件化开发，支持多语言和响应式设计。

**核心特点**:
- Vue 3 Composition API
- Vite 构建工具
- Naive UI 组件库
- Vue I18n 国际化 (8 种语言)
- Pinia 状态管理
- Xterm.js 终端模拟
- ApexCharts 图表
- 自动深色模式

## 2. 项目结构

### 2.1 目录树

```
ui/
├── src/
│   ├── components/              # Vue 组件
│   │   ├── Loading.vue          # 加载卡片
│   │   ├── Information.vue      # 服务器信息
│   │   ├── Speedtest.vue        # 测速组件
│   │   ├── Utilities.vue        # 工具集合
│   │   ├── TrafficDisplay.vue   # 流量显示
│   │   ├── Copy.vue             # 复制按钮
│   │   └── Utilities/           # 工具子组件
│   │       ├── Ping.vue         # Ping 工具
│   │       ├── IPerf3.vue       # iPerf3 工具
│   │       ├── Shell.vue        # Shell 终端
│   │       └── SpeedtestNet.vue # Speedtest.net
│   ├── config/
│   │   └── lang.js              # 多语言配置
│   ├── locales/                 # 翻译文件
│   │   ├── zh-CN.json
│   │   ├── en-US.json
│   │   ├── ru-RU.json
│   │   ├── de-DE.json
│   │   ├── es-AR.json
│   │   ├── fr-FR.json
│   │   ├── ja-JP.json
│   │   └── ko-KR.json
│   ├── stores/
│   │   └── app.js               # 全局状态
│   ├── helper/
│   │   └── unit.js              # 工具函数
│   ├── App.vue                  # 根组件
│   └── main.js                  # 入口文件
├── public/
│   └── speedtest_worker.js      # 测速 Web Worker
├── package.json
├── vite.config.js
└── README.md
```

### 2.2 技术栈

**依赖管理**:
```json
{
  "dependencies": {
    "pinia": "^3.0.4",
    "vue": "^3.5.32"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^6.0.6",
    "@xterm/xterm": "^6.0.0",
    "@xterm/addon-attach": "^0.12.0",
    "@xterm/addon-fit": "^0.11.0",
    "apexcharts": "^5.10.6",
    "axios": "^1.15.0",
    "naive-ui": "^2.44.1",
    "vue-i18n": "^11.3.2",
    "vue3-apexcharts": "^1.11.1",
    "vite": "^8.0.8"
  }
}
```

**核心库说明**:

| 库 | 用途 | 版本 |
|------|------|------|
| Vue 3 | 核心框架 | 3.5.32+ |
| Pinia | 状态管理 | 3.0.4+ |
| Vite | 构建工具 | 8.0.8+ |
| Naive UI | UI 组件库 | 2.44.1+ |
| Vue I18n | 国际化 | 11.3.2+ |
| Xterm.js | 终端模拟 | 6.0.0+ |
| ApexCharts | 图表库 | 5.10.6+ |
| Axios | HTTP 客户端 | 1.15.0+ |

## 3. 构建配置

### 3.1 Vite 配置

**文件**: `ui/vite.config.js`

```javascript
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    vueJsx(),
    AutoImport({
      imports: [
        'vue',
        {
          'naive-ui': [
            'useDialog',
            'useMessage',
            'useNotification',
            'useLoadingBar'
          ]
        }
      ]
    }),
    Components({
      resolvers: [NaiveUiResolver()]
    })
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/session': 'http://localhost:80',
      '/method': 'http://localhost:80',
      '/assets': 'http://localhost:80'
    }
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'pinia', 'vue-i18n'],
          ui: ['naive-ui', '@vicons/carbon'],
          charts: ['apexcharts', 'vue3-apexcharts'],
          terminal: ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-attach']
        }
      }
    }
  }
})
```

**配置说明**:
- **自动导入**: 自动导入 Vue 和 Naive UI API
- **组件自动注册**: Naive UI 组件按需引入
- **路径别名**: `@` 指向 `src` 目录
- **开发代理**: 转发 API 请求到后端
- **代码分割**: 优化加载性能

### 3.2 开发模式

**启动命令**:
```bash
npm install
npm run dev
```

**输出**:
```
VITE v8.0.8  ready in 1234 ms

➜  Local:   http://localhost:5173/
➜  Network: use --host to expose
```

**热重载**:
- 修改组件自动刷新
- 保持应用状态
- CSS 热更新

### 3.3 生产构建

**构建命令**:
```bash
npm run build
```

**输出目录**: `ui/dist/`

**构建物**:
```
dist/
├── index.html
├── assets/
│   ├── index-[hash].js       # 主应用
│   ├── vendor-[hash].js      # 第三方库
│   ├── ui-[hash].js          # UI 组件
│   ├── charts-[hash].js      # 图表库
│   ├── terminal-[hash].js    # 终端组件
│   └── [hash].css           # 样式
└── speedtest_worker.js
```

**预览**:
```bash
npm run preview
```

## 4. 核心组件

### 4.1 根组件 (App.vue)

**职责**:
- 全局配置
- 主题管理
- 语言切换
- 组件布局

**代码结构**:

```vue
<script setup>
import { darkTheme, useOsTheme } from 'naive-ui'
import { computed, ref, onMounted } from 'vue'
import { useAppStore } from './stores/app'
import LoadingCard from '@/components/Loading.vue'
import InfoCard from '@/components/Information.vue'
import SpeedtestCard from '@/components/Speedtest.vue'
import UtilitiesCard from '@/components/Utilities.vue'
import TrafficCard from '@/components/TrafficDisplay.vue'

const currentLangCode = ref('en-US')
const osThemeRef = useOsTheme()
const theme = computed(() => osThemeRef.value === 'dark' ? darkTheme : null)
const appStore = useAppStore()

onMounted(async () => {
  // 自动检测语言
  currentLangCode.value = await autoLang()
})
</script>

<template>
  <n-config-provider :locale="lang" :date-locale="dateLang" :theme="theme">
    <n-global-style />
    <n-message-provider>
      <n-space vertical>
        <h2>{{ $t('app_title') }}</h2>
        
        <!-- 加载卡片 -->
        <LoadingCard v-if="appStore.connecting" />
        
        <!-- 主内容 -->
        <template v-else>
          <InfoCard />
          <UtilitiesCard />
          <SpeedtestCard />
          <TrafficCard v-if="appStore.config.feature_iface_traffic" />
        </template>
        
        <!-- 页脚 -->
        <n-space justify="space-between">
          <div>
            <div>{{ $t('powered_by') }} ALS</div>
            <div>{{ $t('memory_usage') }}: {{ appStore.memoryUsage }}</div>
          </div>
          <div>
            <n-select v-model:value="currentLangCode" :options="langDropdown" />
          </div>
        </n-space>
      </n-space>
    </n-message-provider>
  </n-config-provider>
</template>
```

**特点**:
- 跟随系统深色模式
- 自动语言检测
- 实时内存使用显示

### 4.2 加载组件 (Loading.vue)

**职责**: 显示连接中状态

```vue
<template>
  <n-card>
    <n-spin :show="true">
      <template #description>
        {{ $t('connecting_to_server') }}
      </template>
    </n-spin>
  </n-card>
</template>
```

### 4.3 服务器信息组件 (Information.vue)

**职责**: 显示服务器基本信息

**代码结构**:

```vue
<script setup>
import { useAppStore } from '@/stores/app'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { config } = storeToRefs(appStore)
const { t } = useI18n()

// 计算属性
const location = computed(() => config.value.location || t('unknown'))
const ipv4 = computed(() => config.value.public_ipv4 || t('unknown'))
const ipv6 = computed(() => config.value.public_ipv6 || t('unknown'))
</script>

<template>
  <n-card :title="$t('server_info')">
    <n-descriptions bordered>
      <n-descriptions-item label="位置">
        {{ location }}
      </n-descriptions-item>
      <n-descriptions-item label="IPv4">
        <Copyable :value="ipv4" />
      </n-descriptions-item>
      <n-descriptions-item label="IPv6">
        <Copyable :value="ipv6" />
      </n-descriptions-item>
    </n-descriptions>
  </n-card>
</template>
```

**显示内容**:
- 服务器位置
- IPv4 地址
- IPv6 地址
- 复制功能

### 4.4 测速组件 (Speedtest.vue)

**职责**: 提供多种测速方式

**代码结构**:

```vue
<script setup>
import { useAppStore } from '@/stores/app'
import { computed } from 'vue'
import FileSpeedtest from './Speedtest/FileSpeedtest.vue'
import Librespeed from './Speedtest/Librespeed.vue'

const appStore = useAppStore()
const { config } = storeToRefs(appStore)

const showFileSpeedtest = computed(() => config.value.feature_filespeedtest)
const showLibrespeed = computed(() => config.value.feature_librespeed)
</script>

<template>
  <n-card v-if="showFileSpeedtest || showLibrespeed" :title="$t('speedtest')">
    <n-tabs type="segment">
      <n-tab-pane
        v-if="showFileSpeedtest"
        name="file"
        :tab="$t('file_speedtest')"
      >
        <FileSpeedtest />
      </n-tab-pane>
      <n-tab-pane
        v-if="showLibrespeed"
        name="librespeed"
        :tab="$t('html5_speedtest')"
      >
        <Librespeed />
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>
```

### 4.5 工具集合组件 (Utilities.vue)

**职责**: 提供网络诊断工具入口

**详见**: [组件交互](#5-组件交互)

## 5. 工具子组件

### 5.1 Ping 工具

**路径**: `ui/src/components/Utilities/Ping.vue`

**功能**: 执行 Ping 测试

**代码结构**:

```vue
<script setup>
import { ref } from 'vue'
import axios from 'axios'
import { useAppStore } from '@/stores/app'
import { NCard, NInput, NButton, useMessage } from 'naive-ui'

const emit = defineEmits(['closed'])
const message = useMessage()
const appStore = useAppStore()

const target = ref('8.8.8.8')
const result = ref('')
const loading = ref(false)

const handlePing = async () => {
  if (!target.value) {
    message.error(t('please_enter_target'))
    return
  }

  loading.value = true
  result.value = ''

  try {
    const sessionId = appStore.sessionId
    const response = await fetch(`/method/ping?target=${target.value}`, {
      headers: { 'Session': sessionId }
    })

    if (!response.ok) throw new Error('Ping failed')

    result.value = await response.text()
  } catch (error) {
    message.error(error.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="ping-tool">
    <n-space vertical>
      <n-space>
        <n-input v-model:value="target" placeholder="目标地址" />
        <n-button type="primary" @click="handlePing" :loading="loading">
          Ping
        </n-button>
      </n-space>

      <n-card v-if="result">
        <pre>{{ result }}</pre>
      </n-card>
    </n-space>

    <n-button @click="emit('closed')">关闭</n-button>
  </div>
</template>

<style scoped>
pre {
  white-space: pre-wrap;
  font-family: monospace;
}
</style>
```

**交互流程**:
1. 输入目标地址
2. 点击 Ping 按钮
3. 发送请求到后端
4. 显示 Ping 结果

### 5.2 iPerf3 工具

**路径**: `ui/src/components/Utilities/IPerf3.vue`

**功能**: 获取 iPerf3 服务器连接信息

**代码结构**:

```vue
<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const serverInfo = ref(null)
const loading = ref(false)

const getServerInfo = async () => {
  loading.value = true
  try {
    const response = await fetch('/method/iperf3/server', {
      headers: { 'Session': appStore.sessionId }
    })
    serverInfo.value = await response.json()
  } catch (error) {
    console.error('Failed to get server info:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  getServerInfo()
})
</script>

<template>
  <div class="iperf3-tool">
    <n-spin :show="loading">
      <n-card v-if="serverInfo">
        <n-statistic label="服务器">
          {{ serverInfo.server }}
        </n-statistic>
        <n-statistic label="端口">
          {{ serverInfo.port }}
        </n-statistic>

        <n-space vertical>
          <n-divider title-content="left">
            {{ $t('usage_guide') }}
          </n-divider>

          <n-code :code="`iperf3 -c ${serverInfo.server} -p ${serverInfo.port}`" />
          
          <n-code :code="`iperf3 -c ${serverInfo.server} -p ${serverInfo.port} -R`" />
        </n-space>
      </n-card>
    </n-spin>
  </div>
</template>
```

**显示内容**:
- 服务器地址
- 端口号
- 使用示例

### 5.3 Shell 终端

**路径**: `ui/src/components/Utilities/Shell.vue`

**功能**: 基于 WebSocket 的交互式终端

**技术**:
- Xterm.js 终端模拟
- FitAddon 自适应大小
- AttachAddon WebSocket 连接

**代码结构**:

```vue
<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { useAppStore } from '@/stores/app'
import '@xterm/xterm/css/xterm.css'

const emit = defineEmits(['closed'])
const appStore = useAppStore()

const container = ref(null)
let term = null
let ws = null
let fitAddon = null

onMounted(() => {
  // 初始化终端
  term = new Terminal({
    cursorBlink: true,
    theme: {
      background: '#1e1e1e',
      foreground: '#ffffff'
    },
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    fontSize: 14
  })

  // 自适应插件
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(container.value)
  fitAddon.fit()

  // 连接 WebSocket
  const sessionId = appStore.sessionId
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/session/${sessionId}/shell`
  
  ws = new WebSocket(url)
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    term.writeln('Connected to server.\r\n')
    
    // 窗口调整
    fitAddon.onResize(size => {
      const data = `2${size.rows};${size.cols}`
      ws.send(new TextEncoder().encode(data))
    })
  }

  // 终端输入 -> WebSocket
  term.onData(data => {
    const encoder = new TextEncoder()
    ws.send(encoder.encode('1' + data))
  })

  // WebSocket 输出 -> 终端
  ws.onmessage = (event) => {
    const decoder = new TextDecoder()
    const text = decoder.decode(event.data)
    term.write(text)
  }

  ws.onclose = () => {
    term.writeln('\r\n\r\nDisconnected from server.')
  }

  ws.onerror = () => {
    term.writeln('\r\nConnection error.')
  }
})

onUnmounted(() => {
  if (ws) ws.close()
  if (term) term.dispose()
})

const emitClosed = () => emit('closed')
</script>

<template>
  <div class="shell-tool">
    <div ref="container" class="terminal-container" />
    <n-button @click="emitClosed" style="margin-top: 10px">
      {{ $t('close') }}
    </n-button>
  </div>
</template>

<style scoped>
.terminal-container {
  height: 400px;
  border: 1px solid #333;
  border-radius: 4px;
  padding: 8px;
}
</style>
```

**特性**:
- 支持 ANSI 转义码
- 终端窗口自适应
- 实时交互
- 错误处理

### 5.4 Speedtest.net 工具

**路径**: `ui/src/components/Utilities/SpeedtestNet.vue`

**功能**: 使用 Speedtest CLI 进行测速

**代码结构**:

```vue
<script setup>
import { ref } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const result = ref('')
const loading = ref(false)

const handleSpeedtest = async () => {
  loading.value = true
  result.value = ''

  try {
    const response = await fetch('/method/speedtest_dot_net', {
      headers: { 'Session': appStore.sessionId }
    })

    if (!response.ok) throw new Error('Speedtest failed')

    result.value = await response.text()
  } catch (error) {
    message.error(error.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="speedtest-net-tool">
    <n-button type="primary" @click="handleSpeedtest" :loading="loading">
      {{ $t('start_speedtest') }}
    </n-button>

    <n-card v-if="result" style="margin-top: 10px">
      <pre>{{ result }}</pre>
    </n-card>
  </div>
</template>
```

## 6. 状态管理

### 6.1 App Store

**路径**: `ui/src/stores/app.js`

**职责**: 全局状态管理

**代码结构**:

```javascript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAppStore = defineStore('app', () => {
  // State
  const connecting = ref(true)
  const sessionId = ref(null)
  const config = ref({})
  const memoryUsage = ref('0%')
  const drawerWidth = ref(500)

  // Computed
  const isMobile = computed(() => window.innerWidth < 768)

  // Actions
  const setSessionId = (id) => {
    sessionId.value = id
    connecting.value = false
  }

  const updateConfig = (newConfig) => {
    config.value = { ...config.value, ...newConfig }
  }

  const updateMemoryUsage = (usage) => {
    memoryUsage.value = usage
  }

  return {
    // State
    connecting,
    sessionId,
    config,
    memoryUsage,
    drawerWidth,
    
    // Computed
    isMobile,
    
    // Actions
    setSessionId,
    updateConfig,
    updateMemoryUsage
  }
})
```

**状态说明**:

| 状态 | 类型 | 说明 |
|------|------|------|
| `connecting` | boolean | 连接中状态 |
| `sessionId` | string | 当前会话 ID |
| `config` | object | 服务器配置 |
| `memoryUsage` | string | 内存使用率 |
| `drawerWidth` | number | 侧边栏宽度 |

### 6.2 使用 Store

**组件中**:

```vue
<script setup>
import { useAppStore } from '@/stores/app'
import { storeToRefs } from 'pinia'

const appStore = useAppStore()
const { sessionId, config } = storeToRefs(appStore)

// 修改状态
appStore.setSessionId(uuid)
appStore.updateMemoryUsage('15.3%')

// 访问状态
console.log(sessionId.value)
console.log(config.value.feature_ping)
</script>
```

## 7. 国际化

### 7.1 配置

**路径**: `ui/src/config/lang.js`

**支持语言**:

```javascript
export const list = [
  { label: '简体中文', value: 'zh-CN', autoChangeMap: ['zh-CN', 'zh', 'zh-Hans'] },
  { label: 'English', value: 'en-US', autoChangeMap: ['en-US', 'en', 'en-GB'] },
  { label: 'Русский', value: 'ru-RU', autoChangeMap: ['ru-RU', 'ru'] },
  { label: 'Deutsch', value: 'de-DE', autoChangeMap: ['de-DE', 'de'] },
  { label: 'Español', value: 'es-AR', autoChangeMap: ['es-AR', 'es', 'es-ES'] },
  { label: 'Français', value: 'fr-FR', autoChangeMap: ['fr-FR', 'fr'] },
  { label: '日本語', value: 'ja-JP', autoChangeMap: ['ja-JP', 'ja'] },
  { label: '한국어', value: 'ko-KR', autoChangeMap: ['ko-KR', 'ko'] }
]
```

### 7.2 自动语言检测

```javascript
export async function autoLang() {
  // 1. 检查缓存
  const savedLocale = localStorage.getItem('als-locale')
  if (getLangByCode(savedLocale)) {
    await loadLocaleMessages(savedLocale)
    setI18nLanguage(savedLocale)
    return savedLocale
  }

  // 2. 精确匹配浏览器语言
  const browserLocales = navigator.languages
  for (const browserLocale of browserLocales) {
    for (const lang of list) {
      if (lang.autoChangeMap.includes(browserLocale)) {
        await loadLocaleMessages(lang.value)
        setI18nLanguage(lang.value)
        return lang.value
      }
    }
  }

  // 3. 前缀匹配 (zh-TW -> zh)
  for (const browserLocale of browserLocales) {
    const prefix = browserLocale.split('-')[0]
    for (const lang of list) {
      if (lang.autoChangeMap.some(m => m.split('-')[0] === prefix)) {
        await loadLocaleMessages(lang.value)
        setI18nLanguage(lang.value)
        return lang.value
      }
    }
  }

  // 4. 默认英语
  await loadLocaleMessages(DEFAULT_LOCALE)
  setI18nLanguage(DEFAULT_LOCALE)
  return DEFAULT_LOCALE
}
```

### 7.3 翻译文件结构

**路径**: `ui/src/locales/zh-CN.json`

```json
{
  "app_title": "网络速度测试",
  "server_info": "服务器信息",
  "speedtest": "测速",
  "network_tools": "网络工具",
  "tool_ping": "Ping",
  "tool_iperf3": "iPerf3",
  "tool_shell": "Shell",
  "tool_speedtest_net": "Speedtest.net",
  "connecting_to_server": "正在连接服务器...",
  "powered_by": "由以下技术提供支持：",
  "memory_usage": "内存使用",
  "start_speedtest": "开始测速",
  "close": "关闭",
  "unknown": "未知"
}
```

### 7.4 组件中使用

```vue
<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n({ useScope: 'global' })
</script>

<template>
  <h2>{{ t('app_title') }}</h2>
  <n-button>{{ t('start_speedtest') }}</n-button>
</template>
```

## 8. SSE 连接管理

### 8.1 创建会话

**主组件中**:

```vue
<script setup>
import { onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
let eventSource = null

onMounted(async () => {
  eventSource = new EventSource('/session')

  eventSource.addEventListener('SessionId', (e) => {
    appStore.setSessionId(e.data)
  })

  eventSource.addEventListener('Config', (e) => {
    const config = JSON.parse(e.data)
    appStore.updateConfig(config)
  })

  eventSource.addEventListener('SystemResource', (e) => {
    appStore.updateMemoryUsage(e.data)
  })

  eventSource.addEventListener('InterfaceTraffic', (e) => {
    // 更新流量图表
  })
})

onUnmounted(() => {
  if (eventSource) {
    eventSource.close()
  }
})
</script>
```

### 8.2 错误处理

```javascript
eventSource.onerror = (error) => {
  console.error('SSE error:', error)
  eventSource.close()
  
  // 重连逻辑
  setTimeout(() => {
    location.reload()
  }, 2000)
}
```

## 9. 组件交互

### 9.1 Utilities 组件交互流程

```vue
<script setup>
const tools = ref([
  {
    labelKey: 'tool_ping',
    show: false,
    enable: false,
    configKey: 'feature_ping',
    componentNode: defineAsyncComponent(() => import('./Utilities/Ping.vue'))
  }
])

onMounted(() => {
  for (var tool of tools.value) {
    tool.enable = config.value[tool.configKey] ?? false
  }
})

const toolComponentShow = computed({
  get() {
    for (const tool of tools.value) {
      if (tool.show) {
        return true
      }
    }
    return false
  },
  set(newValue) {
    if (newValue) return
    for (const tool of tools.value) {
      if (tool.show) {
        tool.show = false
        return
      }
    }
  }
})
</script>

<template>
  <n-card v-if="hasToolEnable">
    <n-space>
      <n-button
        v-for="tool in tools"
        v-if="tool.enable"
        @click="tool.show = true"
      >
        {{ t(tool.labelKey) }}
      </n-button>
    </n-space>
  </n-card>

  <n-drawer v-model:show="toolComponentShow">
    <n-drawer-content :title="toolComponentLabel" closable>
      <component :is="toolComponent" @closed="toolComponentShow = false" />
    </n-drawer-content>
  </n-drawer>
</template>
```

**流程**:
1. 根据配置显示可用工具按钮
2. 点击工具按钮打开侧边栏
3. 懒加载对应组件
4. 组件内发送请求到后端

## 10. 性能优化

### 10.1 组件懒加载

```javascript
import { defineAsyncComponent } from 'vue'

const ShellComponent = defineAsyncComponent({
  loader: () => import('./Utilities/Shell.vue'),
  delay: 200,
  timeout: 3000
})
```

**优势**:
- 按需加载
- 减少初始包大小
- 提升首屏速度

### 10.2 代码分割

**Vite 配置**:

```javascript
build: {
  rollupOptions: {
    output: {
      manualChunks: {
        vendor: ['vue', 'pinia', 'vue-i18n'],
        ui: ['naive-ui'],
        charts: ['apexcharts'],
        terminal: ['@xterm/xterm']
      }
    }
  }
}
```

**效果**:
- 分离第三方库
- 并行加载
- 缓存优化

### 10.3 响应式优化

```vue
<script setup>
import { shallowRef } from 'vue'

// 对于大型对象使用 shallowRef
const largeData = shallowRef(null)

// 仅替换引用，不深度监听
largeData.value = newData
</script>
```

## 11. 样式规范

### 11.1 Scoped CSS

```vue
<style scoped>
.container {
  padding: 16px;
}

.title {
  font-size: 24px;
  font-weight: bold;
}
</style>
```

**优势**:
- 样式局部作用域
- 避免命名冲突
- 编译时优化

### 11.2 CSS 变量

```vue
<style scoped>
.card {
  background-color: var(--n-color);
  color: var(--n-text-color);
}
</style>
```

**说明**: 使用 Naive UI 的 CSS 变量，支持主题切换

### 11.3 响应式设计

```vue
<style scoped>
.container {
  display: grid;
  grid-template-columns: 1fr;
}

@media (min-width: 768px) {
  .container {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
```

## 12. 代码规范

### 12.1 ESLint 配置

**文件**: `ui/.eslintrc.cjs`

```javascript
module.exports = {
  extends: [
    'plugin:vue/vue3-recommended',
    'prettier'
  ],
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module'
  }
}
```

### 12.2 Prettier 配置

**文件**: `ui/.prettierrc`

```json
{
  "semi": false,
  "singleQuote": true,
  "trailingComma": "none",
  "printWidth": 80,
  "tabWidth": 2
}
```

### 12.3 代码检查

```bash
# ESLint
npm run lint

# 格式化
npm run format

# 修复自动问题
npm run lint -- --fix
```

## 13. 测试

### 13.1 构建测试

```bash
npm run build
```

**验证**:
- 无编译错误
- 无 ESLint 警告
- 产物完整

### 13.2 手动测试清单

- [ ] 语言切换功能正常
- [ ] 深色模式切换正常
- [ ] SSE 连接稳定
- [ ] WebSocket Shell 连接正常
- [ ] 所有工具按钮点击正常
- [ ] 响应式布局正常

## 14. 故障排查

### 14.1 常见问题

**问题 1**: 前端无法连接后端

**解决**:
```javascript
// 检查 vite.config.js 的 proxy 配置
server: {
  proxy: {
    '/session': 'http://localhost:80'
  }
}
```

**问题 2**: 语言切换不生效

**解决**:
```javascript
// 确保正确加载翻译
await loadLocaleMessages(lang.value)
setI18nLanguage(lang.value)
```

**问题 3**: WebSocket 连接失败

**检查**:
- 会话 ID 是否有效
- URL 协议 (ws/wss)
- CORS 配置

### 14.2 调试技巧

**开发工具**:
- Vue DevTools - 检查组件状态
- Network - 查看请求
- Console - 查看日志

**调试模式**:
```javascript
// 添加调试日志
console.log('Session ID:', sessionId.value)
console.log('Config:', config.value)
```

## 15. 部署

### 15.1 Docker 部署

**Dockerfile 构建阶段**:

```dockerfile
FROM node:lts-alpine AS builder_node_js
ADD ui/package.json /app/package.json
WORKDIR /app
RUN npm i

FROM node:lts-alpine AS builder_node_js_prod
ADD ui /app
WORKDIR /app
COPY --from=builder_node_js /app/node_modules /app/node_modules
RUN npm run build
RUN chmod -R 650 /app/dist
```

### 15.2 嵌入后端

**Go embed**:

```go
//go:embed ui
var UIStaticFiles embed.FS

// 使用时
subFs, _ := fs.Sub(uiFs, "ui")
httpFs := http.FileServer(http.FS(subFs))
```

### 15.3 CDN 部署

**配置**:

```javascript
build: {
  rollupOptions: {
    output: {
      assetFileNames: 'assets/[name]-[hash][extname]',
      entryFileNames: 'assets/[name]-[hash].js'
    }
  }
}
```

## 16. 相关文件

- [系统架构](../ARCHITECTURE.md) - 前端在架构中的位置
- [会话机制](../专有概念/会话机制.md) - 前端使用会话的方式
- [接口文档](../INTERFACES.md) - 前端调用的 API
