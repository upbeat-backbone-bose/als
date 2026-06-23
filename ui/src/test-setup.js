import { config } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const i18n = createI18n({
  legacy: false,
  locale: 'en-US',
  fallbackLocale: 'en-US',
  messages: {
    'en-US': {}
  }
})

config.global.plugins = [i18n]

config.global.mocks = {
  $t: (key) => key
}

// --- Mock EventSource (SSE) ---
// jsdom has no EventSource. Tests can:
//   1. inspect addEventListener calls (for wiring assertions)
//   2. call dispatchEvent(type, data) to simulate an SSE message
//   3. call triggerError() to simulate the onerror reconnect path
class MockEventSource {
  constructor() {
    this.onerror = null
    this.onopen = null
    this.readyState = 0
    this._listeners = {}
  }

  addEventListener(type, handler) {
    if (!this._listeners[type]) {
      this._listeners[type] = []
    }
    this._listeners[type].push(handler)
  }

  removeEventListener(type, handler) {
    if (this._listeners[type]) {
      this._listeners[type] = this._listeners[type].filter((h) => h !== handler)
    }
  }

  close() {
    this.readyState = 2
  }

  // Test helpers
  dispatchEvent(type, data) {
    const handlers = this._listeners[type] || []
    for (const h of handlers) h({ data })
  }

  triggerError() {
    if (this.onerror) this.onerror(new Event('error'))
  }
}

global.EventSource = MockEventSource

// --- Mock navigator.clipboard (used by Copy.vue) ---
// jsdom has no clipboard API. Provide a default no-op + spy-friendly stub.
// Individual tests can override with vi.stubGlobal / Object.defineProperty.
if (!navigator.clipboard) {
  Object.defineProperty(navigator, 'clipboard', {
    value: {
      writeText: () => Promise.resolve()
    },
    writable: true,
    configurable: true
  })
}

window.matchMedia =
  window.matchMedia ||
  function () {
    return {
      matches: false,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {}
    }
  }
