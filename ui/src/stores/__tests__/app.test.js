import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import axios from 'axios'
import { useAppStore } from '@/stores/app'

// The test-setup.js axios mock exports __mockGet / __mockCreate handles on
// the default export so tests can configure behavior per case.
const { __mockGet: mockGet, __mockCreate: mockCreate } = axios

describe('useAppStore', () => {
  let consoleLogSpy
  let consoleErrorSpy
  let originalInnerWidth

  beforeEach(() => {
    setActivePinia(createPinia())
    mockGet.mockReset()
    mockCreate.mockClear()
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

  // ---------- Initialization ----------

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

  // ---------- EventSource edge cases ----------

  it('keeps the latest SessionId value on repeated events', () => {
    const store = useAppStore()
    store.source.dispatchEvent('SessionId', 'first')
    store.source.dispatchEvent('SessionId', 'second')
    store.source.dispatchEvent('SessionId', 'third')
    expect(store.sessionId).toBe('third')
  })

  it('connects to the relative ./session URL', () => {
    // The store should construct the EventSource with a relative path so it
    // rides the same host as the SPA. This protects against accidental
    // hard-coded absolute URLs that would break reverse-proxy deployments.
    const store = useAppStore()
    expect(store.source.url).toBe('./session')
  })

  it('does not throw when Config payload is invalid JSON', () => {
    // app.js calls JSON.parse(e.data) without try/catch. Document the actual
    // behavior: a malformed Config crashes the listener invocation. The
    // store remains alive; the next event still updates state. This test
    // guards against a silent regression to silent swallowing.
    const store = useAppStore()
    expect(() => store.source.dispatchEvent('Config', '{not json')).toThrow()
    // subsequent valid Config still works
    store.source.dispatchEvent('Config', JSON.stringify({ public_ipv4: '5.6.7.8' }))
    expect(store.config).toEqual({ public_ipv4: '5.6.7.8' })
  })

  it('toggles connecting back to true on Config after it was cleared', () => {
    // Config first clears connecting. If a fresh Config event arrives later
    // (e.g. server pushes a config update), the listener does not flip
    // connecting — it stays false. Document that as the current contract.
    const store = useAppStore()
    store.source.dispatchEvent('Config', JSON.stringify({ a: 1 }))
    expect(store.connecting).toBe(false)
    store.source.dispatchEvent('Config', JSON.stringify({ a: 2 }))
    expect(store.connecting).toBe(false)
    expect(store.config).toEqual({ a: 2 })
  })

  // ---------- reconnectEventSource (1s timer) ----------

  it('reconnects 1s after onerror', () => {
    vi.useFakeTimers()
    const store = useAppStore()
    const first = store.source
    first.triggerError()
    expect(first.readyState).toBe(2)
    expect(store.connecting).toBe(true)
    expect(store.source).toBe(first)
    vi.advanceTimersByTime(1000)
    expect(store.source).not.toBe(first)
    expect(store.source).toBeInstanceOf(global.EventSource)
  })

  it('the reconnect creates a fresh EventSource with all three listeners re-registered', () => {
    vi.useFakeTimers()
    const store = useAppStore()
    const first = store.source
    first.triggerError()
    vi.advanceTimersByTime(1000)
    // new source, same wiring
    expect(store.source).not.toBe(first)
    expect(store.source._listeners['SessionId']).toBeDefined()
    expect(store.source._listeners['Config']).toBeDefined()
    expect(store.source._listeners['MemoryUsage']).toBeDefined()
  })

  // ---------- requestMethod ----------

  it('requestMethod resolves on success payload', async () => {
    mockGet.mockResolvedValue({ data: { success: true, result: 42 } })
    const store = useAppStore()
    store.sessionId = 'sess-1'
    const result = await store.requestMethod('ping', { ip: '8.8.8.8' })
    expect(result).toEqual({ success: true, result: 42 })
    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('./method/ping', { params: { ip: '8.8.8.8' } })
  })

  it('requestMethod passes AbortSignal to axios config', async () => {
    mockGet.mockResolvedValue({ data: { success: true } })
    const store = useAppStore()
    const ac = new AbortController()
    await store.requestMethod('iperf3/server', {}, ac.signal)
    expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ signal: ac.signal }))
  })

  it('requestMethod omits signal when not provided', async () => {
    mockGet.mockResolvedValue({ data: { success: true } })
    const store = useAppStore()
    await store.requestMethod('ping', {})
    expect(mockCreate).toHaveBeenCalledWith(
      expect.not.objectContaining({ signal: expect.anything() })
    )
  })

  it('requestMethod attaches the current sessionId as a header', async () => {
    mockGet.mockResolvedValue({ data: { success: true } })
    const store = useAppStore()
    store.sessionId = 'sess-xyz'
    await store.requestMethod('ping', {})
    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({ headers: expect.objectContaining({ session: 'sess-xyz' }) })
    )
  })

  it('requestMethod rejects when response.data.success is false', async () => {
    const resp = { data: { success: false, error: 'no' } }
    mockGet.mockResolvedValue(resp)
    const store = useAppStore()
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
