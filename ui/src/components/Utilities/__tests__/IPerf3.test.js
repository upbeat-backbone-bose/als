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
  // The Terminal constructor is mocked at the module level. Each instance is
  // a fresh mock. We pull the most recent one by inspecting mock.calls.
  const calls = Terminal.mock.calls
  if (calls.length === 0) return null
  return Terminal.mock.results[calls.length - 1].value
}

describe('IPerf3.vue (SSE)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Terminal.mockClear()
    // IPerf3.vue reads appStore.config.public_ipv4 / public_ipv6 in its
    // template. The store starts with config = undefined, which would
    // raise "Cannot read properties of undefined" on first render. Set a
    // minimal config so the template's optional guards are exercised.
    const store = useAppStore()
    store.config = { public_ipv4: '1.2.3.4', public_ipv6: '' }
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

    // The component sets port.value = e.data on the Iperf3 listener.
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
    // Reset writeln call count to isolate this case
    term.writeln.mockClear()

    const store = useAppStore()
    store.source.dispatchEvent('Iperf3Stream', 'line one\nline two\nline three')
    await flushPromises()

    // The handler splits on \n and writelns each line.
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
})
