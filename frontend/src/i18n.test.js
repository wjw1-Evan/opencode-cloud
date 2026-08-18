// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import { useI18n } from './i18n'

describe('i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    // The locale ref is a module singleton; reset it for isolation.
    useI18n().setLocale('zh')
  })

  it('defaults to Chinese and translates zh keys', () => {
    const { locale, t } = useI18n()
    expect(locale.value).toBe('zh')
    expect(t('login')).toBe('进入平台')
  })

  it('switches locale and persists the choice', () => {
    const { locale, setLocale, t } = useI18n()
    setLocale('en')
    expect(locale.value).toBe('en')
    expect(localStorage.getItem('devcapsule-locale')).toBe('en')
    expect(t('login')).toBe('Sign In')
  })

  it('falls back to the key itself for unknown keys', () => {
    const { t } = useI18n()
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('interpolates template params', () => {
    const { t } = useI18n()
    expect(t('selectedCount', { n: 3 })).toBe('已选 3 个用户')
    expect(t('createPartial', { n: 3, f: 1, err: 'boom' })).toContain('已创建 3 个账号')
    expect(t('createPartial', { n: 3, f: 1, err: 'boom' })).toContain('1 个容器创建失败')
  })

  it('returns the raw value when no params are provided', () => {
    const { t } = useI18n()
    expect(t('login', null)).toBe('进入平台')
  })
})
