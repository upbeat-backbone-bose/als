import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock axios BEFORE importing the store. vi.mock is hoisted, but we
// still define the mock factory first so it's available at module load.
const mockGet = vi.fn()
const mockAxiosInstance = { get: mockGet }

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => mockAxiosInstance)
  }
}))

import { useAppStore } from '@/stores/app'

describe('useAppStore', () => {
  let consoleLogSpy
  let consoleErrorSpy
  let originalInnerWidth

  beforeEach(() => {
    setActivePinia(createPinia())
    mockGet.mockReset()
    // silence noisy expected console output
    consoleLogSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    originalInnerWidth = window.innerWidth
    // Ensure deterministic drawerWidth
    Object.defineProperty(window, 'innerWidth', { value: 1024, writable: true, configurable: true })
  })

  afterEach(() => {
    consoleLogSpy.mockRestore()
    consoleErrorSpy.mockRestore()
    Object.defineProperty(window, 'innerWidth', { value: originalInnerWidth, writable: true, configurable: true })
    vi.useRealTimers()
  })

  // ---------- Initialization (existing) ----------

  it('initializes with connecting as true', () => {
    const store = useAppStore()
    expect(store.connecting).toBe(true)
  })

  it('initializes with null sessionId', () => {
    const store = useAppStore()
    expect(store.sessionId).toBeUndefined()
  })

  it('initializes with null config', () => {
    const store = useAppStore()
    expect(store.config).toBeUndefined()
  })

  it('exposes requestMethod function', () => {
    const store = useAppStore()
    expect(typeof store.requestMethod).toBe('function')
  })

  it('initializes drawerWidth based on window width', () => {
    const store = useAppStore()
    expect(typeof store.drawerWidth).toBe('number')
  })

  // ---------- EventSource wiring ----------

  it('creates an EventSource on store init', () => {
    // store creation already triggered setupEventSource() in app.js
    const store = useAppStore()
    expect(store.source).toBeDefined()
    expect(store.source).toBeInstanceOf(global.EventSource)
  })

  it('registers SessionId / Config / MemoryUsage listeners on the source', () => {
    const store = useAppStore()
    expect(store.source._listeners['SessionId']).toBeDefined()
    expect(store.source._listeners['Config']).toBeDefined()
    expect(store.source._listeners['MemoryUsage']).toBeDefined()
  })

  it('updates sessionId on SessionId event', () => {
    const store = useAppStore()
    store.source.dispatchEvent('SessionId', 'abc-123')
    expect(store.sessionId).toBe('abc-123')
  })

  it('updates config and clears connecting on Config event', () => {
    const store = useAppStore()
    const payload = JSON.stringify({ public_ipv4: '1.2.3.4' })
    store.source.dispatchEvent('Config', payload)
    expect(store.config).toEqual({ public_ipv4: '1.2.3.4' })
    expect(store.connecting).toBe(false)
  })

  it('formats memoryUsage via formatBytes on MemoryUsage event', () => {
    const store = useAppStore()
    // formatBytes(0) === '0 Bytes'; formatBytes(1024) === '1 KB'
    store.source.dispatchEvent('MemoryUsage', 1024)
    expect(store.memoryUsage).toBe('1 KB')
  })

  // ---------- reconnectEventSource (1s timer) ----------

  it('reconnects 1s after onerror', () => {
    vi.useFakeTimers()
    const store = useAppStore()
    const first = store.source
    // trigger SSE error → store calls close + reconnectEventSource
    first.triggerError()
    expect(first.readyState).toBe(2) // closed
    expect(store.connecting).toBe(true)
    // before the timer fires, source is still the old one
    expect(store.source).toBe(first)
    // advance 1 second
    vi.advanceTimersByTime(1000)
    // a fresh EventSource is created and assigned
    expect(store.source).not.toBe(first)
    expect(store.source).toBeInstanceOf(global.EventSource)
  })

  // ---------- requestMethod ----------

  it('requestMethod resolves on success payload', async () => {
    mockGet.mockResolvedValue({ data: { success: true, result: 42 } })
    const store = useAppStore()
    store.sessionId = 'sess-1'
    const result = await store.requestMethod('ping', { ip: '8.8.8.8' })
    expect(result).toEqual({ success: true, result: 42 })
    // axios.create is called once per request
    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('./method/ping', { params: { ip: '8.8.8.8' } })
  })

  it('requestMethod passes AbortSignal to axios config', async () => {
    mockGet.mockResolvedValue({ data: { success: true } })
    const store = useAppStore()
    const ac = new AbortController()
    await store.requestMethod('iperf3/server', {}, ac.signal)
    // axios.create was called with the signal
    const axios = (await import('axios')).default
    expect(axios.create).toHaveBeenCalledWith(
      expect.objectContaining({ signal: ac.signal })
    )
  })

  it('requestMethod omits signal when not provided', async () => {
    mockGet.mockResolvedValue({ data: { success: true } })
    const store = useAppStore()
    await store.requestMethod('ping', {})
    const axios = (await import('axios')).default
    expect(axios.create).toHaveBeenCalledWith(
      expect.not.objectContaining({ signal: expect.anything() })
    )
  })

  it('requestMethod rejects when response.data.success is false', async () => {
    const resp = { data: { success: false, error: 'no' } }
    mockGet.mockResolvedValue(resp)
    const store = useAppStore()
    // The store rejects with the whole axios response object (not response.data)
    await expect(store.requestMethod('ping')).rejects.toBe(resp)
  })

  it('requestMethod rejects on ERR_CANCELED without console.error', async () => {
    const cancelErr = Object.assign(new Error('aborted'), { code: 'ERR_CANCELED' })
    mockGet.mockRejectedValue(cancelErr)
    const store = useAppStore()
    await expect(store.requestMethod('ping')).rejects.toBe(cancelErr)
    expect(consoleErrorSpy).not.toHaveBeenCalled()
  })

  it('requestMethod logs and rejects on generic error', async () => {
    const boom = new Error('network down')
    mockGet.mockRejectedValue(boom)
    const store = useAppStore()
    await expect(store.requestMethod('ping')).rejects.toBe(boom)
    expect(consoleErrorSpy).toHaveBeenCalledWith(boom)
  })
})
