import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import axios from 'axios'
import Ping from '@/components/Utilities/Ping.vue'

const { __mockGet: mockGet } = axios

// Ping.vue registers its SSE listener just before awaiting requestMethod(),
// so we keep the request in flight to dispatch SSE events mid-ping.
function startInFlightPing() {
  let resolveRequest
  const pending = new Promise((r) => {
    resolveRequest = r
  })
  mockGet.mockReset()
  mockGet.mockImplementation(() => pending)
  return {
    resolve: () => resolveRequest({ data: { success: true } }),
    promise: pending
  }
}

describe('Ping.vue (SSE)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function createWrapper() {
    return mount(Ping)
  }

  it('adds a record when a Ping event arrives mid-ping', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightPing()
    await wrapper.find('input').setValue('1.1.1.1')
    await wrapper.find('button').trigger('click')
    await flushPromises()

    const store = useAppStore()
    store.source.dispatchEvent(
      'Ping',
      JSON.stringify({ seq: 1, from: '1.1.1.1', ttl: 64, latency: 1000000, is_timeout: false })
    )
    await flushPromises()

    // n-table uses v-show so the element stays in the DOM. Assert on html().
    const html = wrapper.html()
    expect(html).toContain('1.1.1.1')
    expect(html).toContain('>64<')
    // latency is rendered as `1.00 ms` via record.latency.toFixed(2)
    expect(html).toContain('1.00 ms')

    inflight.resolve()
    await flushPromises()
  })

  it('leaves the table hidden when no Ping events have arrived', () => {
    // The v-show directive keeps the element in the DOM but applies
    // display: none when records.length is 0. We assert the table is
    // present but hidden.
    const wrapper = createWrapper()
    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.find('table').attributes('style')).toContain('display: none')
  })

  it('removes the Ping listener on component unmount', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightPing()
    await wrapper.find('input').setValue('8.8.8.8')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    // While the ping is in flight, the listener is registered
    expect(store.source._listeners['Ping']).toBeDefined()
    expect(store.source._listeners['Ping'].length).toBeGreaterThanOrEqual(1)
    // Unmount triggers onUnmounted -> stopPing -> removeEventListener
    wrapper.unmount()
    // After cleanup, the listener array exists but is empty (removeEventListener
    // filters the array rather than deleting the key).
    expect(store.source._listeners['Ping']).toEqual([])
    inflight.resolve()
  })

  it('removes the Ping listener after the request resolves naturally', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightPing()
    await wrapper.find('input').setValue('8.8.8.8')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    expect(store.source._listeners['Ping'].length).toBeGreaterThanOrEqual(1)
    // Resolving the request lets ping() complete; the `finally`-style cleanup
    // (stopPing + working = false) removes the listener.
    inflight.resolve()
    await flushPromises()
    expect(store.source._listeners['Ping']).toEqual([])
  })

  // KNOWN BUG: Ping.vue's template calls record.latency.toFixed(2) without
  // checking the latency value. When the backend sends a timeout event the
  // handler sets latency to the string '-', and string.toFixed(2) throws
  // TypeError during render. The test below documents the current behavior
  // by checking the data that the handler wrote before the render attempt.
  it('handler stores "-" as latency on timeout (template will fail to render)', async () => {
    // Install a Vue error handler so the toFixed(2) TypeError on render
    // does not fail the test; we are only verifying the handler's data path.
    const errs = []
    const wrapper = createWrapper()
    wrapper.vm.$.appContext.app.config.errorHandler = (err) => errs.push(err)
    const inflight = startInFlightPing()
    await wrapper.find('input').setValue('1.1.1.1')
    await wrapper.find('button').trigger('click')
    await flushPromises()

    const store = useAppStore()
    store.source.dispatchEvent('Ping', JSON.stringify({ seq: 1, is_timeout: true }))
    await flushPromises()

    // The handler should have pushed a record with latency = '-'. We can
    // inspect records via the next tick's component instance since the
    // template threw before rendering. The component's `records` ref is
    // accessible through the wrapped setup proxy.
    // Note: a render error means the v-for may not have run, but the
    // ref was mutated by the SSE handler.
    // We use a separate component instance to read the records field:
    const records = wrapper.vm.$?.setupState?.records ?? wrapper.vm.records
    expect(records).toBeDefined()
    expect(records.length).toBe(1)
    expect(records[0].latency).toBe('-')

    // The render error path produced at least one toFixed TypeError.
    const hasToFixedError = errs.some((e) => /toFixed/.test(String(e.message)))
    expect(hasToFixedError).toBe(true)

    inflight.resolve()
  })
})
