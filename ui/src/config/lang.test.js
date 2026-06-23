import { describe, it, expect } from 'vitest'
import { list, DEFAULT_LOCALE, getLangByCode } from '@/config/lang.js'

describe('lang.js', () => {
  it('has a default locale of en-US', () => {
    expect(DEFAULT_LOCALE).toBe('en-US')
  })

  it('has 8 supported languages', () => {
    expect(list).toHaveLength(8)
  })

  it('contains all expected language codes', () => {
    const codes = list.map((item) => item.value)
    expect(codes).toContain('zh-CN')
    expect(codes).toContain('en-US')
    expect(codes).toContain('ru-RU')
    expect(codes).toContain('de-DE')
    expect(codes).toContain('es-AR')
    expect(codes).toContain('fr-FR')
    expect(codes).toContain('ja-JP')
    expect(codes).toContain('ko-KR')
  })

  it('each language has required properties', () => {
    for (const lang of list) {
      expect(lang).toHaveProperty('label')
      expect(lang).toHaveProperty('value')
      expect(lang).toHaveProperty('autoChangeMap')
      expect(Array.isArray(lang.autoChangeMap)).toBe(true)
      expect(typeof lang.uiLang).toBe('function')
      expect(typeof lang.dateLang).toBe('function')
    }
  })

  it('autoChangeMap for en-US includes en', () => {
    const enLang = list.find((item) => item.value === 'en-US')
    expect(enLang.autoChangeMap).toContain('en')
    expect(enLang.autoChangeMap).toContain('en-US')
  })

  it('autoChangeMap for zh-CN includes zh', () => {
    const zhLang = list.find((item) => item.value === 'zh-CN')
    expect(zhLang.autoChangeMap).toContain('zh')
    expect(zhLang.autoChangeMap).toContain('zh-CN')
  })

  it('getLangByCode returns the correct language object', () => {
    const en = getLangByCode('en-US')
    expect(en.label).toBe('English')

    const zh = getLangByCode('zh-CN')
    expect(zh.label).toBe('简体中文')
  })

  it('getLangByCode returns null for invalid code', () => {
    expect(getLangByCode('invalid')).toBeNull()
  })
})
