import { ref } from 'vue'
import { defineStore } from 'pinia'
import axios from 'axios'
import { formatBytes } from '@/helper/unit'
export const useAppStore = defineStore('app', () => {
  const source = ref()
  const sessionId = ref()
  // `connecting` is only true on the very first connection, before
  // we have ever received a `Config` event. Once we have, transient
  // SSE drops must NOT toggle the loading screen back on -- flipping
  // `connecting` would unmount every <InfoCard/>, <UtilitiesCard/>,
  // <SpeedtestCard/>, <TrafficCard/> in App.vue's v-else branch and
  // look like a full page refresh to the user.
  const connecting = ref(true)
  // `ready` flips to true the first time the server sends us a
  // `Config` event. From then on we treat reconnects as transparent.
  const ready = ref(false)
  const config = ref()
  const drawerWidth = ref()
  const memoryUsage = ref()
  let timer = ''

  const handleResize = () => {
    let width = window.innerWidth
    if (width > 800) {
      drawerWidth.value = 800
    } else {
      drawerWidth.value = width
    }
  }
  window.addEventListener('resize', handleResize)
  handleResize()

  const reconnectEventSource = () => {
    clearTimeout(timer)
    setTimeout(() => {
      setupEventSource()
    }, 1000)
  }

  const setupEventSource = () => {
    // Only the very first attempt should show the loading screen.
    // After we have at least one successful `Config` event, SSE
    // reconnects happen in-place without touching the UI.
    if (!ready.value) {
      connecting.value = true
    }
    const eventSource = new EventSource('./session')

    eventSource.addEventListener('SessionId', (e) => {
      sessionId.value = e.data
      console.log('session', e.data)
    })

    eventSource.addEventListener('Config', (e) => {
      config.value = JSON.parse(e.data)
      ready.value = true
      connecting.value = false
    })

    eventSource.addEventListener('MemoryUsage', (e) => {
      memoryUsage.value = formatBytes(e.data)
    })

    eventSource.onerror = function () {
      eventSource.close()
      // Only flip back to the loading state if we never connected
      // successfully. Otherwise the user just sees a slightly stale
      // page for ~1s while we reconnect.
      if (!ready.value) {
        connecting.value = true
      }
      console.log('SSE disconnected')
      reconnectEventSource()
    }
    source.value = eventSource
  }

  setupEventSource()

  const requestMethod = (method, data = {}, signal = null) => {
    let axiosConfig = {
      timeout: 1000 * 120, // 请求超时时间
      headers: {
        session: sessionId.value
      }
    }

    if (signal != null) {
      axiosConfig.signal = signal
    }

    const _axios = axios.create(axiosConfig)

    return new Promise((resolve, reject) => {
      _axios
        .get('./method/' + method, { params: data })
        .then((response) => {
          if (response.data.success) {
            resolve(response.data)
            return
          }
          reject(response)
        })
        .catch((error) => {
          if (error.code == 'ERR_CANCELED') {
            reject(error)
            return
          }
          console.error(error)
          reject(error)
        })
    })
  }

  return {
    //vars
    source,
    sessionId,
    connecting,
    config,
    drawerWidth,
    memoryUsage,

    //methods
    requestMethod
  }
})
