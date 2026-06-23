import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const { useMessageMock, messageApi } = vi.hoisted(() => {
  const info = vi.fn()
  return {
    useMessageMock: vi.fn(() => ({ info })),
    messageApi: { info }
  }
})

vi.mock('naive-ui', async (importOriginal) => {
  const original = await importOriginal()
  return {
    ...original,
    useMessage: useMessageMock
  }
})

import { mount } from '@vue/test-utils'
import Copy from '@/components/Copy.vue'

describe('Copy.vue', () => {
  let writeTextSpy
  let execCommandSpy
  let execCommandDescriptor

  beforeEach(() => {
    messageApi.info.mockReset()
    // Default: clipboard.writeText resolves. Tests can override per-case.
    writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    // jsdom's `document` does not have an `execCommand` property; installing a
    // spy via vi.spyOn(document, 'execCommand') throws. Define the property
    // manually so the spy can intercept the fallback path in Copy.vue.
    execCommandSpy = vi.fn(() => true)
    execCommandDescriptor = {
      configurable: true,
      writable: true,
      value: execCommandSpy
    }
    Object.defineProperty(document, 'execCommand', execCommandDescriptor)
  })

  afterEach(() => {
    writeTextSpy.mockRestore()
    if (execCommandDescriptor) {
      delete document.execCommand
    }
  })

  // ---------- Existing rendering tests ----------

  it('renders slot content in text mode', () => {
    const wrapper = mount(Copy, {
      props: { text: true, value: 'test-value' },
      slots: { default: 'Copy me' }
    })
    expect(wrapper.text()).toContain('Copy me')
    expect(wrapper.find('button').exists()).toBe(true)
  })

  it('renders slot content in tooltip mode', () => {
    const wrapper = mount(Copy, {
      props: { text: false, value: 'test-value' },
      slots: { default: 'Copy me' }
    })
    expect(wrapper.text()).toContain('Copy me')
  })

  it('text mode uses n-button for copy action', () => {
    const wrapper = mount(Copy, {
      props: { text: true, value: 'test-value' },
      slots: { default: 'Click' }
    })
    expect(wrapper.find('button').exists()).toBe(true)
    expect(wrapper.text()).toBe('Click')
  })

  // ---------- Click / clipboard behavior ----------

  it('clicking in text mode calls navigator.clipboard.writeText with the value', async () => {
    const wrapper = mount(Copy, {
      props: { text: true, value: 'hello-world' }
    })
    await wrapper.find('button').trigger('click')
    expect(writeTextSpy).toHaveBeenCalledWith('hello-world')
  })

  it('falls back to document.execCommand when clipboard.writeText throws', async () => {
    writeTextSpy.mockRejectedValueOnce(new Error('not allowed'))
    const wrapper = mount(Copy, {
      props: { text: true, value: 'fallback-value' }
    })
    await wrapper.find('button').trigger('click')
    // Give the async copy() chain a tick to settle
    await new Promise((r) => setTimeout(r, 0))
    expect(writeTextSpy).toHaveBeenCalledWith('fallback-value')
    expect(execCommandSpy).toHaveBeenCalledWith('copy')
  })

  it('shows a "copied_to_clipboard" message by default', async () => {
    const wrapper = mount(Copy, {
      props: { text: true, value: 'x' }
    })
    await wrapper.find('button').trigger('click')
    expect(messageApi.info).toHaveBeenCalledWith('copied_to_clipboard')
  })

  it('does NOT show a message when hideMessage is true', async () => {
    const wrapper = mount(Copy, {
      props: { text: true, value: 'x', hideMessage: true }
    })
    await wrapper.find('button').trigger('click')
    expect(messageApi.info).not.toHaveBeenCalled()
  })
})
