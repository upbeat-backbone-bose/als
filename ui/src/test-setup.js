import { config } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { vi } from 'vitest'

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

// --- Default axios mock ---
// The store and several components call axios.create().get(...). Real network
// is never desired in unit tests, so we provide a no-op mock by default.
// Tests that care about specific axios behavior import this module and
// override `__mockGet` / `__mockCreate` between cases.
const defaultMockGet = vi.fn().mockResolvedValue({ data: { success: true } })
const defaultMockAxiosInstance = { get: defaultMockGet }
const defaultMockCreate = vi.fn(() => defaultMockAxiosInstance)

vi.mock('axios', () => ({
  default: {
    create: defaultMockCreate,
    __mockGet: defaultMockGet,
    __mockCreate: defaultMockCreate
  }
}))

// --- Default xterm mocks ---
// IPerf3.vue and Shell.vue import xterm at module load (`new Terminal()` in
// setup runs synchronously). jsdom cannot construct the real Terminal, so
// the modules are stubbed with minimal interfaces. Individual tests can
// override via vi.mocked(...).mockImplementation or replace the import.
vi.mock('@xterm/xterm', () => {
  // Components instantiate `new Terminal()`, so the mock must be a callable
  // constructor. We build it as a regular function and let vi.fn() wrap it
  // so per-instance calls are still tracked via .mock.results.
  const Terminal = vi.fn(function () {
    return {
      loadAddon: vi.fn(),
      open: vi.fn(),
      writeln: vi.fn(),
      write: vi.fn(),
      clear: vi.fn(),
      onData: vi.fn(),
      onResize: vi.fn()
    }
  })
  return { Terminal }
})

vi.mock('@xterm/addon-fit', () => {
  const FitAddon = vi.fn(function () {
    return { fit: vi.fn(), proposeDimensions: vi.fn() }
  })
  return { FitAddon }
})

// --- Mock EventSource (SSE) ---
// jsdom has no EventSource. Tests can:
//   1. inspect addEventListener calls (for wiring assertions)
//   2. call dispatchEvent(type, data) to simulate an SSE message
//   3. call triggerError() to simulate the onerror reconnect path
class MockEventSource {
  constructor(url) {
    this.url = url
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
