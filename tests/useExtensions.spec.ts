// @vitest-environment happy-dom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useExtensions } from '../src/useExtensions'

const { getFileUrl, showErrorMessage, useClientService, useMessages } = vi.hoisted(() => {
  const getFileUrl = vi.fn()
  const showErrorMessage = vi.fn()
  const showMessage = vi.fn()

  return {
    getFileUrl,
    showErrorMessage,
    showMessage,
    useClientService: vi.fn(() => ({ webdav: { getFileUrl } })),
    useMessages: vi.fn(() => ({ showErrorMessage, showMessage }))
  }
})

vi.mock('@opencloud-eu/web-pkg', () => ({
  useClientService,
  useMessages
}))

vi.mock('vue3-gettext', () => ({
  useGettext: () => ({ $gettext: (message: string) => message })
}))

const configuredOptions = () => ({
  applicationConfig: { simpledmsBaseUrl: 'https://simpledms.example.com/' }
})

const resource = (overrides: Record<string, unknown> = {}) => ({
  isFolder: false,
  canDownload: () => true,
  ...overrides
})

const getAction = (options = configuredOptions()) => useExtensions(options as never).value[0].action

describe('useExtensions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the action only for one configured downloadable file', () => {
    const action = getAction()
    const file = resource()

    expect(action.isVisible({ resources: [file] } as never)).toBe(true)
    expect(action.isVisible({ resources: [resource({ isFolder: true })] } as never)).toBe(false)
    expect(action.isVisible({ resources: [resource({ canDownload: () => false })] } as never)).toBe(
      false
    )
    expect(action.isVisible({ resources: [file, resource()] } as never)).toBe(false)
    expect(
      getAction({ applicationConfig: { simpledmsBaseUrl: '' } }).isVisible({
        resources: [file]
      } as never)
    ).toBe(false)
  })

  it('gets the attachment URL and opens the encoded SimpleDMS target in a new tab', async () => {
    const file = resource()
    const space = { id: 'space' }
    const downloadUrl = 'https://cloud.example.com/file.pdf?signature=a+b&expires=123'
    const openedWindow = { opener: window }
    getFileUrl.mockResolvedValue(downloadUrl)
    vi.spyOn(window, 'open').mockReturnValue(openedWindow as Window)

    await getAction().handler({ space, resources: [file] } as never)

    expect(getFileUrl).toHaveBeenCalledWith(space, file, { disposition: 'attachment' })
    const target =
      'https://simpledms.example.com/open-file/from-url?url=' + encodeURIComponent(downloadUrl)
    expect(window.open).toHaveBeenCalledWith(target, '_blank')
    expect(openedWindow.opener).toBeNull()
  })

  it('surfaces download failures through the error message service', async () => {
    const failure = new Error('download failed')
    getFileUrl.mockRejectedValue(failure)

    await getAction().handler({ space: {}, resources: [resource()] } as never)

    expect(showErrorMessage).toHaveBeenCalledWith({
      title: 'Upload to SimpleDMS failed',
      errors: [failure]
    })
  })
})
