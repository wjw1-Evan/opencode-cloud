// @vitest-environment happy-dom
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { inject, ref } from 'vue'
import App from './App.vue'

function mountApp() {
  const locale = ref('zh')
  const setLocale = vi.fn((l) => { locale.value = l })
  const wrapper = mount(App, {
    global: {
      provide: { i18n: { locale, setLocale, t: (key) => key } },
      stubs: { 'router-view': true },
    },
  })
  return { wrapper, setLocale }
}

describe('App', () => {
  it('renders the language switcher with Chinese active by default', () => {
    const { wrapper } = mountApp()
    const buttons = wrapper.findAll('.lang-btn')
    expect(buttons.length).toBe(2)
    expect(buttons[0].classes()).toContain('active')
  })

  it('switches language through the switcher buttons', async () => {
    const { wrapper, setLocale } = mountApp()
    const buttons = wrapper.findAll('.lang-btn')
    await buttons[1].trigger('click')
    expect(setLocale).toHaveBeenCalledWith('en')
    await wrapper.findAll('.lang-btn')[0].trigger('click')
    expect(setLocale).toHaveBeenCalledWith('zh')
  })

  it('shows a toast notification with the given type and hides it after 3s', async () => {
    vi.useFakeTimers()
    const Trigger = {
      template: '<button id="trig" @click="go">go</button>',
      setup() {
        const notify = inject('notify')
        return { go: () => notify('hello', 'err') }
      },
    }
    const wrapper = mount(App, {
      global: {
        provide: { i18n: { locale: ref('zh'), setLocale: vi.fn(), t: (key) => key } },
        stubs: { 'router-view': Trigger },
      },
    })

    await wrapper.find('#trig').trigger('click')
    const toast = wrapper.find('.toast')
    expect(toast.exists()).toBe(true)
    expect(toast.classes()).toContain('err')
    expect(toast.text()).toBe('hello')

    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.find('.toast').exists()).toBe(false)
    vi.useRealTimers()
  })
})
