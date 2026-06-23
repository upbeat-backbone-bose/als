import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import Speedtest from '@/components/Speedtest.vue'

// Speedtest.vue explicitly imports its subcomponents (Librespeed, FileSpeedtest)
// so we stub those by PascalCase name. naive-ui components (NCard, NDivider) are
// auto-resolved by the test plugin chain and render real DOM; we assert on
// their stable CSS classes rather than stubbing.
describe('Speedtest.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function createWrapper() {
    return mount(Speedtest, {
      global: {
        stubs: {
          Librespeed: { template: '<div class="librespeed-stub" />' },
          FileSpeedtest: { template: '<div class="filespeedtest-stub" />' }
        }
      }
    })
  }

  function setConfig(overrides) {
    const store = useAppStore()
    store.config = { feature_librespeed: false, feature_filespeedtest: false, ...overrides }
  }

  it('hides the entire card when neither feature flag is set', () => {
    setConfig({ feature_librespeed: false, feature_filespeedtest: false })
    const wrapper = createWrapper()
    expect(wrapper.html()).not.toMatch(/class="n-card[\s"]/)
  })

  it('shows the card with Librespeed when feature_librespeed is true', () => {
    setConfig({ feature_librespeed: true })
    const wrapper = createWrapper()
    expect(wrapper.html()).toMatch(/class="n-card[\s"]/)
    expect(wrapper.find('.librespeed-stub').exists()).toBe(true)
    expect(wrapper.find('.filespeedtest-stub').exists()).toBe(false)
  })

  it('shows the card with FileSpeedtest when feature_filespeedtest is true', () => {
    setConfig({ feature_filespeedtest: true })
    const wrapper = createWrapper()
    expect(wrapper.html()).toMatch(/class="n-card[\s"]/)
    expect(wrapper.find('.filespeedtest-stub').exists()).toBe(true)
    expect(wrapper.find('.librespeed-stub').exists()).toBe(false)
  })

  it('shows the divider between Librespeed and FileSpeedtest when both are enabled', () => {
    setConfig({ feature_librespeed: true, feature_filespeedtest: true })
    const wrapper = createWrapper()
    expect(wrapper.html()).toMatch(/class="n-divider[\s"]/)
  })

  it('omits the divider when only one feature is enabled', () => {
    setConfig({ feature_librespeed: true, feature_filespeedtest: false })
    const wrapper = createWrapper()
    expect(wrapper.html()).not.toMatch(/class="n-divider[\s"]/)
  })
})
