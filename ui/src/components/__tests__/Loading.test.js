import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Loading from '@/components/Loading.vue'

// Loading.vue uses <n-card> and <n-progress> directly (auto-resolved by the
// test plugin chain via unplugin-vue-components + NaiveUiResolver). They render
// real DOM with stable naive-ui CSS classes and ARIA attributes, so we assert
// on those markers rather than trying to stub the components.

describe('Loading.vue', () => {
  function createWrapper() {
    return mount(Loading)
  }

  it('renders successfully', () => {
    const wrapper = createWrapper()
    expect(wrapper.exists()).toBe(true)
  })

  it('renders an n-card component', () => {
    const wrapper = createWrapper()
    // naive-ui renders NCard as <div class="n-card ...">; the trailing space
    // avoids matching a custom class like "n-card-foo".
    expect(wrapper.html()).toMatch(/class="n-card[\s"]/)
  })

  it('renders with loading title', () => {
    const wrapper = createWrapper()
    expect(wrapper.text()).toContain('loading')
  })

  it('renders an n-progress with type="line" and 100%', () => {
    const wrapper = createWrapper()
    const html = wrapper.html()
    // n-progress is mounted
    expect(html).toMatch(/class="n-progress[\s"]/)
    // type="line" produces n-progress--line
    expect(html).toContain('n-progress--line')
    // percentage=100 is exposed as aria-valuenow="100" on the progressbar role
    expect(html).toContain('aria-valuenow="100"')
  })
})
