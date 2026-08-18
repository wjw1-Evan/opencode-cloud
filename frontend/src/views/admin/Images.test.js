// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }))

vi.mock('../../api', () => ({
  api: {
    listImages: vi.fn(),
    getImage: vi.fn(),
    deleteImage: vi.fn(),
    uploadImage: vi.fn(),
    pullImage: vi.fn(),
  },
  fmtBytes: (n) => `${n} B`,
}))

import { api } from '../../api'
import Images from './Images.vue'

const images = [
  { id: 'sha256:aaaa1111', repo_tags: ['nginx:latest'], size: 1024, created: 1700000000, in_use: true, used_by: ['user-stu001'] },
  { id: 'sha256:bbbb2222', repo_tags: ['<none>:<none>'], size: 2048, created: 1700000001, in_use: false, used_by: [] },
]

function mountImages(list = images) {
  api.listImages.mockResolvedValue(list)
  return mount(Images, {
    global: {
      provide: { i18n: { t: (key) => key }, notify: notifyMock },
      stubs: { teleport: true },
    },
  })
}

describe('Images', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    notifyMock.mockClear()
  })

  it('renders images with usage status and disables delete for in-use images', async () => {
    const wrapper = mountImages()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(2)
    expect(rows[0].find('.badge').classes()).toContain('inuse')
    expect(rows[1].find('.badge').classes()).toContain('free')
    expect(rows[0].find('.btn-danger').attributes('disabled')).toBeDefined()
    expect(rows[1].find('.btn-danger').attributes('disabled')).toBeUndefined()
  })

  it('shows the in-use referrers in the badge tooltip', async () => {
    const wrapper = mountImages()
    await flushPromises()

    expect(wrapper.find('tbody tr .badge').attributes('title')).toBe('inUseBy')
  })

  it('deletes an unused image after confirmation', async () => {
    api.deleteImage.mockResolvedValue({})
    const wrapper = mountImages()
    await flushPromises()

    const unusedRow = wrapper.findAll('tbody tr')[1]
    await unusedRow.find('.btn-danger').trigger('click')
    expect(wrapper.findComponent(ConfirmDialog).props('visible')).toBe(true)

    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(api.deleteImage).toHaveBeenCalledWith('sha256:bbbb2222')
  })

  it('cancelling the delete confirmation keeps the image', async () => {
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('tbody tr')[1].find('.btn-danger').trigger('click')
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[0].trigger('click')
    expect(api.deleteImage).not.toHaveBeenCalled()
  })

  it('imports a selected file', async () => {
    api.uploadImage.mockResolvedValue({ message: 'ok' })
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('.head-btns .btn')[0].trigger('click') // upload
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { value: [new File(['x'], 'a.tar')] })
    await fileInput.trigger('change')
    await wrapper.find('.modal .btns .btn-primary').trigger('click') // import
    await flushPromises()

    expect(api.uploadImage).toHaveBeenCalled()
    expect(wrapper.find('.modal-mask').exists()).toBe(false)
  })

  it('imports nothing without a selected file', async () => {
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('.head-btns .btn')[0].trigger('click')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()
    expect(api.uploadImage).not.toHaveBeenCalled()
  })

  it('pulls a remote image', async () => {
    api.pullImage.mockResolvedValue({})
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('.head-btns .btn')[1].trigger('click') // pull
    await wrapper.find('.modal input').setValue('nginx:latest')
    await wrapper.findAll('.modal .btns .btn')[0].trigger('click') // pull button
    await flushPromises()

    expect(api.pullImage).toHaveBeenCalledWith('nginx:latest')
    expect(wrapper.find('.modal-mask').exists()).toBe(false)
  })

  it('shows the error when the image detail request fails', async () => {
    api.getImage.mockRejectedValue(new Error('image gone'))
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('image gone', 'err')
  })

  it('shows the error when the list refresh fails', async () => {
    api.listImages.mockRejectedValue(new Error('docker down'))
    const wrapper = mount(Images, {
      global: {
        provide: { i18n: { t: (key) => key }, notify: notifyMock },
        stubs: { teleport: true },
      },
    })
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('docker down', 'err')
  })

  it('closes modals via the mask and cancel buttons and opens the file picker', async () => {
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('.head-btns .btn')[0].trigger('click') // upload
    await wrapper.find('.modal .btn').trigger('click') // choose file button
    await wrapper.find('.modal-mask').trigger('click') // mask
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.findAll('.head-btns .btn')[0].trigger('click')
    await wrapper.findAll('.modal .btns .btn')[1].trigger('click') // cancel
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.findAll('.head-btns .btn')[1].trigger('click') // pull
    await wrapper.find('.modal-mask').trigger('click')
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.find('tbody tr .ops .btn').trigger('click') // detail
    await flushPromises()
    await wrapper.find('.modal-mask').trigger('click')
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.find('tbody tr .ops .btn').trigger('click')
    await flushPromises()
    await wrapper.find('.modal .btns .btn').trigger('click') // close
    expect(wrapper.find('.modal-mask').exists()).toBe(false)
    wrapper.unmount()
  })

  it('opens the detail modal with full image metadata', async () => {
    api.getImage.mockResolvedValue({
      id: 'sha256:aaaa1111',
      repo_tags: ['nginx:latest'],
      repo_digests: ['nginx@sha256:dddd'],
      architecture: 'arm64',
      os: 'linux',
      size: 1024,
      created: 'not-a-date',
      user: '',
      exposed_ports: ['80/tcp'],
      volumes: ['/data'],
      stop_signal: '',
      healthcheck: '',
      env: ['A=B'],
      cmd: ['nginx'],
      entrypoint: [],
      working_dir: '/',
      labels: { maintainer: 'nginx' },
      layers: [],
      in_use: true,
      used_by: ['user-stu001'],
    })
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn').trigger('click')
    await flushPromises()
    expect(wrapper.find('.detail-modal').exists()).toBe(true)
    expect(wrapper.text()).toContain('arm64')
    expect(wrapper.text()).toContain('80/tcp')
    expect(wrapper.text()).toContain('nginx@sha256:dddd')
    expect(wrapper.text()).toContain('/data')
    expect(wrapper.text()).toContain('A=B')
    expect(wrapper.find('.label-key').text()).toBe('maintainer')
  })

  it('shows the error when deleting an image fails', async () => {
    api.deleteImage.mockRejectedValue(new Error('in use'))
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('tbody tr')[1].find('.btn-danger').trigger('click')
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('in use', 'err')
  })

  it('shows the error when importing fails', async () => {
    api.uploadImage.mockRejectedValue(new Error('bad tar'))
    const wrapper = mountImages()
    await flushPromises()

    await wrapper.findAll('.head-btns .btn')[0].trigger('click')
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { value: [new File(['x'], 'a.tar')] })
    await fileInput.trigger('change')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('bad tar', 'err')
  })

  it('formats list timestamps into relative labels', async () => {
    const now = Math.floor(Date.now() / 1000)
    const recent = [
      { id: 'x1', repo_tags: ['a'], size: 1, created: now - 30, in_use: false, used_by: [] },
      { id: 'x2', repo_tags: ['b'], size: 1, created: now - 300, in_use: false, used_by: [] },
      { id: 'x3', repo_tags: ['c'], size: 1, created: now - 18000, in_use: false, used_by: [] },
      { id: 'x4', repo_tags: ['d'], size: 1, created: now - 500000, in_use: false, used_by: [] },
    ]
    const wrapper = mountImages(recent)
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('justNow')
    expect(text).toContain('minutesAgo')
    expect(text).toContain('hoursAgo')
    expect(text).toContain('daysAgo')
    expect(wrapper.find('.free').attributes('title')).toBe('notInUse')
  })

})
