import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'

describe('useAppStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

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
})
