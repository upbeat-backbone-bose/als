import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import axios from 'axios'
import SpeedtestNet from '@/components/Utilities/SpeedtestNet.vue'

const { __mockGet: mockGet } = axios

function startInFlightSpeedtest() {
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

function dispatchSpeedtestEvent(store, payload) {
  store.source.dispatchEvent('SpeedtestStream', JSON.stringify(payload))
}

describe('SpeedtestNet.vue (SSE)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function createWrapper() {
    return mount(SpeedtestNet)
  }

  it('registers the SpeedtestStream listener only after speedtest() is invoked', async () => {
    const wrapper = createWrapper()
    const store = useAppStore()
    expect(store.source._listeners['SpeedtestStream']).toBeUndefined()

    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(store.source._listeners['SpeedtestStream']).toBeDefined()
    expect(store.source._listeners['SpeedtestStream'].length).toBeGreaterThanOrEqual(1)

    inflight.resolve()
    await flushPromises()
  })

  it('"queue" case sets queueStat and isQueue', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, { type: 'queue', pos: 3, totalPos: 10 })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.isQueue).toBe(true)
    expect(setup.queueStat.pos).toBe(3)
    expect(setup.queueStat.total).toBe(10)
    inflight.resolve()
  })

  it('"testStart" case populates serverInfo and flips isSpeedtest on', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, {
      type: 'testStart',
      server: { id: 's-1', name: 'Example ISP', country: 'US', location: 'NYC' }
    })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.isSpeedtest).toBe(true)
    expect(setup.speedtestData.serverInfo.id).toBe('s-1')
    expect(setup.speedtestData.serverInfo.name).toBe('Example ISP')
    expect(setup.speedtestData.serverInfo.pos).toBe('US - NYC')
    inflight.resolve()
  })

  it('"ping" case sets the action label and the measured latency', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, { type: 'ping', ping: { latency: '12ms' } })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.action).toBe('speedtest_latency_action')
    expect(setup.speedtestData.ping).toBe('12ms')
    inflight.resolve()
  })

  it('"download" case updates progress (sub = round(progress*100), full = sub/2)', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, {
      type: 'download',
      download: { bandwidth: 125000000, progress: 0.4 }
    })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.action).toBe('download')
    expect(setup.progress.sub).toBe(40) // round(0.4 * 100)
    expect(setup.progress.full).toBe(20) // 40 / 2
    inflight.resolve()
  })

  it('"upload" case sets full = 50 + sub/2 (total-progress pivot)', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, {
      type: 'upload',
      upload: { bandwidth: 125000000, progress: 0.6 }
    })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.action).toBe('upload')
    expect(setup.progress.sub).toBe(60)
    expect(setup.progress.full).toBe(80) // 50 + 60/2
    inflight.resolve()
  })

  it('"result" case writes the result url and final speeds', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    dispatchSpeedtestEvent(store, {
      type: 'result',
      result: { url: 'https://example.com/r/abc' },
      download: { bandwidth: 125000000 },
      upload: { bandwidth: 62500000 }
    })
    await flushPromises()
    const setup = wrapper.vm.$.setupState
    expect(setup.speedtestData.result).toBe('https://example.com/r/abc')
    expect(setup.speedtestData.download).toMatch(/Mbps/)
    expect(setup.speedtestData.upload).toMatch(/Mbps/)
    inflight.resolve()
  })

  it('removes the SpeedtestStream listener on component unmount', async () => {
    const wrapper = createWrapper()
    const inflight = startInFlightSpeedtest()
    await wrapper.find('input').setValue('12345')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const store = useAppStore()
    expect(store.source._listeners['SpeedtestStream'].length).toBeGreaterThanOrEqual(1)
    wrapper.unmount()
    expect(store.source._listeners['SpeedtestStream']).toEqual([])
    inflight.resolve()
  })
})
