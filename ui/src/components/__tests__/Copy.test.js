import { describe, it, expect, vi } from 'vitest'

vi.mock('naive-ui', async (importOriginal) => {
  const original = await importOriginal()
  return {
    ...original,
    useMessage: vi.fn(() => ({
      info: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
      error: vi.fn(),
      loading: vi.fn()
    }))
  }
})

import { mount } from '@vue/test-utils'
import Copy from '@/components/Copy.vue'

describe('Copy.vue', () => {
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
})
