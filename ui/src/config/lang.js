import {
  zhCN,
  dateZhCN,
  enUS,
  dateEnUS,
  ruRU,
  dateRuRU,
  deDE,
  dateDeDE,
  esAR,
  dateEsAR,
  frFR,
  dateFrFR,
  jaJP,
  dateJaJP,
  koKR,
  dateKoKR
} from 'naive-ui'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'

export const DEFAULT_LOCALE = 'en-US'
const LOCALE_STORAGE_KEY = 'als-locale'

export const list = [
  {
    label: '简体中文',
    value: 'zh-CN',
    autoChangeMap: ['zh-CN', 'zh', 'zh-Hans', 'zh-SG'],
    uiLang: () => zhCN,
    dateLang: () => dateZhCN
  },
  {
    label: 'English',
    value: 'en-US',
    autoChangeMap: ['en-US', 'en', 'en-GB', 'en-CA', 'en-AU'],
    uiLang: () => enUS,
    dateLang: () => dateEnUS
  },
  {
    label: 'Русский',
    value: 'ru-RU',
    autoChangeMap: ['ru-RU', 'ru'],
    uiLang: () => ruRU,
    dateLang: () => dateRuRU
  },
  {
    label: 'Deutsch',
    value: 'de-DE',
    autoChangeMap: ['de-DE', 'de', 'de-AT', 'de-CH'],
    uiLang: () => deDE,
    dateLang: () => dateDeDE
  },
  {
    label: 'Español',
    value: 'es-AR',
    autoChangeMap: ['es-AR', 'es', 'es-ES', 'es-MX', 'es-CL', 'es-CO'],
    uiLang: () => esAR,
    dateLang: () => dateEsAR
  },
  {
    label: 'Français',
    value: 'fr-FR',
    autoChangeMap: ['fr-FR', 'fr', 'fr-CA', 'fr-BE', 'fr-CH'],
    uiLang: () => frFR,
    dateLang: () => dateFrFR
  },
  {
    label: '日本語',
    value: 'ja-JP',
    autoChangeMap: ['ja-JP', 'ja'],
    uiLang: () => jaJP,
    dateLang: () => dateJaJP
  },
  {
    label: '한국어',
    value: 'ko-KR',
    autoChangeMap: ['ko-KR', 'ko'],
    uiLang: () => koKR,
    dateLang: () => dateKoKR
  }
]

export const getLangByCode = (locale) => {
  return list.find((item) => item.value === locale) ?? null
}

const i18n = createI18n({
  locale: DEFAULT_LOCALE,
  fallbackLocale: DEFAULT_LOCALE,
  legacy: false,
  globalInjection: true
})

// copy from https://vue-i18n.intlify.dev/guide/advanced/lazy.html
export async function setupI18n() {
  await loadLocaleMessages(DEFAULT_LOCALE)
  setI18nLanguage(DEFAULT_LOCALE)

  return i18n
}

export function setI18nLanguage(locale) {
  const normalizedLocale = getLangByCode(locale)?.value ?? DEFAULT_LOCALE
  if (i18n.mode === 'legacy') {
    i18n.global.locale = normalizedLocale
  } else {
    i18n.global.locale.value = normalizedLocale
  }
  document.querySelector('html').setAttribute('lang', normalizedLocale)
  localStorage.setItem(LOCALE_STORAGE_KEY, normalizedLocale)
}

const loadedLocales = new Set()

export async function loadLocaleMessages(locale) {
  const normalizedLocale = getLangByCode(locale)?.value ?? DEFAULT_LOCALE
  if (loadedLocales.has(normalizedLocale)) return

  const messages = await import(`../locales/${normalizedLocale}.json`)
  i18n.global.setLocaleMessage(normalizedLocale, messages.default)
  loadedLocales.add(normalizedLocale)

  return nextTick()
}

export async function autoLang() {
  const savedLocale = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (getLangByCode(savedLocale)) {
    await loadLocaleMessages(savedLocale)
    setI18nLanguage(savedLocale)
    return savedLocale
  }

  const browserLocales = navigator.languages?.length ? navigator.languages : [navigator.language]

  // Exact match first
  for (const browserLocale of browserLocales) {
    for (const lang of list) {
      if (lang.autoChangeMap.includes(browserLocale)) {
        await loadLocaleMessages(lang.value)
        setI18nLanguage(lang.value)
        return lang.value
      }
    }
  }

  // Prefix fallback (e.g. 'zh-TW' -> 'zh' matches 'zh-CN')
  for (const browserLocale of browserLocales) {
    const prefix = browserLocale.split('-')[0]
    for (const lang of list) {
      if (lang.autoChangeMap.some((m) => m.split('-')[0] === prefix)) {
        await loadLocaleMessages(lang.value)
        setI18nLanguage(lang.value)
        return lang.value
      }
    }
  }

  await loadLocaleMessages(DEFAULT_LOCALE)
  setI18nLanguage(DEFAULT_LOCALE)
  return DEFAULT_LOCALE
}
