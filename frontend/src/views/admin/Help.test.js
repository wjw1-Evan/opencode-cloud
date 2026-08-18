// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Help from './Help.vue'

const i18nStub = { t: (key) => key }

function mountHelp() {
  return mount(Help, {
    global: { provide: { i18n: i18nStub } },
  })
}

describe('Help', () => {
  it('renders every help section with its content', () => {
    const wrapper = mountHelp()
    const cards = wrapper.findAll('.card')
    // 平台简介 / 三步开课 / 用户与容器 / 镜像模板 / 镜像管理 / 账号状态 / 常见问题
    expect(cards.length).toBe(7)
    expect(wrapper.findAll('ol li').length).toBe(3)
    const faqCard = wrapper.findAll('.card')[6]
    expect(faqCard.findAll('li').length).toBe(5)
    expect(wrapper.find('.intro-desc').text()).toBe('helpIntroDesc')
  })
})
