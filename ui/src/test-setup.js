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

class MockEventSource {
  constructor() {
    this.onerror = null
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

  close() {}
}

global.EventSource = MockEventSource

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
