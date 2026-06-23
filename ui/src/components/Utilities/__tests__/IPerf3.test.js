import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import axios from 'axios'
import { Terminal } from '@xterm/xterm'
import IPerf3 from '@/components/Utilities/IPerf3.vue'

const { __mockGet: mockGet } = axios

function startInFlightServer() {
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

function getLatestTerminal() {
  const calls = Terminal.mock.calls
  if (calls.length === 0) return null
  return Terminal.mock.results[calls.length - 1].value
}

describe('IPerf3.vue (SSE)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Terminal.mockClear()
    // No pre-set store.config: IPerf3.vue must guard against appStore.config
    // being undefined. The fix uses optional chaining in the v-if guards
    // so the alert body stays safe when Config SSE has not arrived yet.
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function createWrapper() {
    return mount(IPerf3, {
      global: {
        stubs: {
          // IPerf3 embeds the Copy component, which calls useMessage().
          // Stub Copy to a no-op placeholder so the message-provider
          // dependency is not triggered in this test file.
          Copy: { template: '<span class="copy-stub" />' }
        }
      }
    })
  }

  it('registers both Iperf3 and Iperf3Stream listeners only after startServer()', async () => {
    const wrapper = createWrapper()
    const store = useAppStore()
    expect(store.source._listeners['Iperf3']).toBeUndefined()
    expect(store.source._listeners['Iperf3Stream']).toBeUndefined()

    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(store.source._listeners['Iperf3']).toBeDefined()
    expect(store.source._listeners['Iperf3Stream']).toBeDefined()
    expect(store.source._listeners['Iperf3'].length).toBeGreaterThanOrEqual(1)
    expect(store.source._listeners['Iperf3Stream'].length).toBeGreaterThanOrEqual(1)

    inflight.resolve()
    await flushPromises()
  })

  it('updates port ref when an Iperf3 event arrives', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    const store = useAppStore()
    store.source.dispatchEvent('Iperf3', '5201')
    await flushPromises()

    const setupState = wrapper.vm.$.setupState
    expect(setupState.port).toBe('5201')

    inflight.resolve()
    await flushPromises()
  })

  it('writes Iperf3Stream lines to the terminal via writeln', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    const term = getLatestTerminal()
    expect(term).not.toBeNull()
    term.writeln.mockClear()

    const store = useAppStore()
    store.source.dispatchEvent('Iperf3Stream', 'line one\nline two\nline three')
    await flushPromises()

    expect(term.writeln).toHaveBeenCalledTimes(3)
    expect(term.writeln).toHaveBeenNthCalledWith(1, 'line one')
    expect(term.writeln).toHaveBeenNthCalledWith(2, 'line two')
    expect(term.writeln).toHaveBeenNthCalledWith(3, 'line three')

    inflight.resolve()
    await flushPromises()
  })

  it('removes both Iperf3 and Iperf3Stream listeners on component unmount', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    const store = useAppStore()
    expect(store.source._listeners['Iperf3'].length).toBeGreaterThanOrEqual(1)
    expect(store.source._listeners['Iperf3Stream'].length).toBeGreaterThanOrEqual(1)

    wrapper.unmount()
    expect(store.source._listeners['Iperf3']).toEqual([])
    expect(store.source._listeners['Iperf3Stream']).toEqual([])
    inflight.resolve()
  })

  // Regression test for the IPerf3.vue config-undefined crash. The alert
  // body reads appStore.config.public_ipv4 / public_ipv6; with config
  // still undefined (no Config SSE yet) and a non-empty port, the alert
  // block must not throw and the body must render without the IP
  // templates (the per-IP v-if guards correctly evaluate to false).
  it('renders the alert safely when port is set but config is undefined', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    // port set by the SSE event
    store.source.dispatchEvent('Iperf3', '5201')
    await flushPromises()
    // The alert mounts (working=true, port='5201'). Before the fix this
    // threw "Cannot read properties of undefined (reading 'public_ipv4')"
    // because the v-if guard accessed the property directly. After the
    // fix (optional chaining) the inner v-if templates simply do not
    // render. Verify no Copy stub is present.
    expect(wrapper.find('.copy-stub').exists()).toBe(false)
    inflight.resolve()
  })

  it('renders the IPv4 Copy template once config arrives', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightServer()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    // arrive config + port
    store.config = { public_ipv4: '1.2.3.4', public_ipv6: '' }
    store.source.dispatchEvent('Iperf3', '5201')
    await flushPromises()
    // The Copy stub template is rendered for the IPv4 path
    expect(wrapper.find('.copy-stub').exists()).toBe(true)
    inflight.resolve()
  })
})
