import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Loading from '@/components/Loading.vue'

describe('Loading.vue', () => {
  function createWrapper() {
    return mount(Loading, {
      global: {
        stubs: {
          'n-card': true,
          'n-progress': true
        }
      }
    })
  }

  it('renders successfully', () => {
    const wrapper = createWrapper()
    expect(wrapper.exists()).toBe(true)
  })

  it('renders an n-card component', () => {
    const wrapper = createWrapper()
    expect(wrapper.html()).toMatch(/n-card|n-card--bordered/)
  })

  it('renders with loading title', () => {
    const wrapper = createWrapper()
    expect(wrapper.text()).toContain('loading')
  })
})
