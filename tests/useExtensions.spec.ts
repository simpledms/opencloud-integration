// @vitest-environment happy-dom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useExtensions } from '../src/useExtensions'

const { showErrorMessage, useAuthStore, useMessages, fetchMock } = vi.hoisted(() => {
  const fetchMock = vi.fn()
  const showErrorMessage = vi.fn()
  const showMessage = vi.fn()
  const useAuthStore = vi.fn(() => ({
    accessToken: 'session-token',
    publicLinkContextReady: false
  }))
  return {
    fetchMock,
    showErrorMessage,
    useAuthStore,
    useMessages: vi.fn(() => ({ showErrorMessage, showMessage }))
  }
})

vi.mock('@opencloud-eu/web-pkg', () => ({ useAuthStore, useMessages }))
vi.mock('vue3-gettext', () => ({ useGettext: () => ({ $gettext: (message: string) => message }) }))

const configuredOptions = () => ({
  applicationConfig: { simpledmsBaseUrl: 'https://simpledms.example.com/' }
})
const resource = (overrides: Record<string, unknown> = {}) => ({
  id: 'file-123',
  isFolder: false,
  canDownload: () => true,
  ...overrides
})
const getAction = (options = configuredOptions()) => useExtensions(options as never).value[0].action
const response = (downloadUrl: string, ok = true) => ({
  ok,
  json: vi.fn().mockResolvedValue({ downloadUrl })
})
const companionUrl = () =>
  `${window.location.origin}/apps/simpledms_integration/download/${'a'.repeat(64)}`

describe('useExtensions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.mockReturnValue({ accessToken: 'session-token', publicLinkContextReady: false })
    vi.stubGlobal('fetch', fetchMock)
    fetchMock.mockResolvedValue(response(companionUrl()))
    vi.spyOn(window, 'open').mockReturnValue({
      opener: window,
      closed: false,
      close: vi.fn(),
      location: { replace: vi.fn() }
    } as never)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('shows only for one authenticated, private, downloadable file', () => {
    const action = getAction()
    expect(
      action.isVisible({ space: { driveType: 'personal' }, resources: [resource()] } as never)
    ).toBe(true)
    expect(action.isVisible({ resources: [] } as never)).toBe(false)
    expect(action.isVisible({ resources: [resource({ isFolder: true })] } as never)).toBe(false)
    expect(action.isVisible({ resources: [resource({ isInVault: true })] } as never)).toBe(false)
    expect(action.isVisible({ resources: [resource({ canDownload: () => false })] } as never)).toBe(
      false
    )
    expect(
      action.isVisible({ space: { driveType: 'public' }, resources: [resource()] } as never)
    ).toBe(false)
    useAuthStore.mockReturnValue({ accessToken: '', publicLinkContextReady: false } as never)
    expect(getAction().isVisible({ resources: [resource()] } as never)).toBe(false)
    useAuthStore.mockReturnValue({ accessToken: 'token', publicLinkContextReady: true } as never)
    expect(getAction().isVisible({ resources: [resource()] } as never)).toBe(false)
  })

  it('posts only the file ID to the fixed same-origin companion and navigates the reserved tab', async () => {
    const opened = { opener: window, closed: false, close: vi.fn(), location: { replace: vi.fn() } }
    vi.mocked(window.open).mockReturnValue(opened as never)
    fetchMock.mockResolvedValue(response(companionUrl()))
    const file = resource({
      fileId: 'cloud-file',
      url: 'https://cloud.example/private?secret=must-not-leak'
    })

    await getAction().handler({ space: { driveType: 'personal' }, resources: [file] } as never)

    expect(fetchMock).toHaveBeenCalledWith(
      '/apps/simpledms_integration/api/create-signed-url',
      expect.objectContaining({
        method: 'POST',
        credentials: 'omit',
        headers: { Authorization: 'Bearer session-token', 'Content-Type': 'application/json' },
        body: JSON.stringify({ fileId: 'cloud-file' })
      })
    )
    expect(opened.location.replace).toHaveBeenCalledWith(
      expect.stringContaining(
        encodeURIComponent('/apps/simpledms_integration/download/' + 'a'.repeat(64))
      )
    )
    expect(opened.location.replace.mock.calls[0][0]).not.toContain('cloud.example')
    expect(opened.opener).toBeNull()
    expect(vi.mocked(window.open).mock.invocationCallOrder[0]).toBeLessThan(
      fetchMock.mock.invocationCallOrder[0]
    )
  })

  it.each([
    'https://evil.example/apps/simpledms_integration/download/' + 'a'.repeat(64),
    'http://localhost/apps/simpledms_integration/download/' + 'a'.repeat(63) + '?secret=leak',
    'http://localhost/apps/simpledms_integration/download/' + 'a'.repeat(64) + '/extra'
  ])('rejects an invalid companion URL %s without sending it to SimpleDMS', async (downloadUrl) => {
    const opened = { closed: false, close: vi.fn(), opener: window, location: { replace: vi.fn() } }
    vi.mocked(window.open).mockReturnValue(opened as never)
    fetchMock.mockResolvedValue(response(downloadUrl))
    await getAction().handler({
      space: { driveType: 'personal' },
      resources: [resource()]
    } as never)
    expect(opened.close).toHaveBeenCalled()
    expect(opened.location.replace).not.toHaveBeenCalled()
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).toMatch(
      /Could not prepare the file/
    )
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).not.toContain(downloadUrl)
  })

  it('uses the fallback navigation when popup blocking closes the reserved tab', async () => {
    vi.mocked(window.open).mockReturnValue(null)
    const assign = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined)
    await getAction().handler({
      space: { driveType: 'personal' },
      resources: [resource()]
    } as never)
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/open-file/from-url'))
  })

  it('closes the reserved tab and reports a generic error when issuance fails', async () => {
    const opened = { closed: false, close: vi.fn(), opener: window }
    vi.mocked(window.open).mockReturnValue(opened as never)
    fetchMock.mockRejectedValue(new Error('upstream-secret-url'))
    await getAction().handler({
      space: { driveType: 'personal' },
      resources: [resource()]
    } as never)
    expect(opened.close).toHaveBeenCalled()
    expect(showErrorMessage).toHaveBeenCalledWith({
      title: 'Upload to SimpleDMS failed',
      errors: [expect.any(Error)]
    })
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).not.toContain('upstream-secret-url')
  })

  it('rechecks visibility in the handler', async () => {
    await getAction().handler({
      space: { driveType: 'personal' },
      resources: [resource({ canDownload: () => false })]
    } as never)
    expect(fetchMock).not.toHaveBeenCalled()
    expect(window.open).not.toHaveBeenCalled()
  })
})
