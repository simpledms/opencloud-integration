// @vitest-environment happy-dom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useExtensions } from '../src/useExtensions'

const {
  createLink,
  deletePermission,
  showErrorMessage,
  useAuthStore,
  useClientService,
  useMessages
} = vi.hoisted(() => {
  const createLink = vi.fn()
  const deletePermission = vi.fn()
  const showErrorMessage = vi.fn()
  const showMessage = vi.fn()
  const useAuthStore = vi.fn(() => ({
    accessToken: 'session-token',
    publicLinkContextReady: false
  }))
  return {
    createLink,
    deletePermission,
    showErrorMessage,
    useAuthStore,
    useClientService: vi.fn(() => ({
      graphAuthenticated: { permissions: { createLink, deletePermission } }
    })),
    useMessages: vi.fn(() => ({ showErrorMessage, showMessage }))
  }
})

vi.mock('@opencloud-eu/web-pkg', () => ({ useAuthStore, useClientService, useMessages }))
vi.mock('vue3-gettext', () => ({ useGettext: () => ({ $gettext: (message: string) => message }) }))

const configuredOptions = () => ({
  applicationConfig: {
    simpledmsBaseUrl: 'https://simpledms.example.com/',
    opencloudPublicLinkPassword: 'Shared-secret-1!'
  }
})
const resource = (overrides: Record<string, unknown> = {}) => ({
  id: 'file-123',
  name: 'report.pdf',
  path: '/Projects/Quarterly/report.pdf',
  isFolder: false,
  canDownload: () => true,
  ...overrides
})
const getAction = (options = configuredOptions()) => useExtensions(options as never).value[0].action
const share = (overrides: Record<string, unknown> = {}) => ({
  id: 'permission-1',
  webUrl: `${window.location.origin}/s/random_token`,
  ...overrides
})
const space = { id: 'drive-1', driveType: 'personal' }

describe('useExtensions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.mockReturnValue({ accessToken: 'session-token', publicLinkContextReady: false })
    createLink.mockResolvedValue(share())
    deletePermission.mockResolvedValue(undefined)
    vi.spyOn(window, 'open').mockReturnValue({
      opener: window,
      closed: false,
      close: vi.fn(),
      location: { replace: vi.fn() }
    } as never)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows only for one authenticated, private, downloadable file', () => {
    const action = getAction()
    expect(action.isVisible({ space, resources: [resource()] } as never)).toBe(true)
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

  it('creates a read-only public link and navigates the reserved tab to SimpleDMS', async () => {
    const opened = { opener: window, closed: false, close: vi.fn(), location: { replace: vi.fn() } }
    vi.mocked(window.open).mockReturnValue(opened as never)
    const file = resource({ url: 'https://cloud.example/private?secret=must-not-leak' })

    await getAction().handler({ space, resources: [file] } as never)

    expect(createLink).toHaveBeenCalledWith(
      'drive-1',
      'file-123',
      expect.objectContaining({
        type: 'view',
        password: 'Shared-secret-1!',
        displayName: 'SimpleDMS export'
      })
    )
    expect(opened.location.replace).toHaveBeenCalledWith(
      expect.stringContaining('source=opencloud')
    )
    expect(opened.location.replace.mock.calls[0][0]).toContain(
      encodeURIComponent('/remote.php/dav/public-files/random_token/report.pdf')
    )
    expect(opened.location.replace.mock.calls[0][0]).toContain(
      'file_path=%2FProjects%2FQuarterly%2Freport.pdf'
    )
    expect(opened.opener).toBe(window)
    expect(vi.mocked(window.open).mock.invocationCallOrder[0]).toBeLessThan(
      createLink.mock.invocationCallOrder[0]
    )
  })

  it.each([
    'https://evil.example/s/random_token',
    `${window.location.origin}/s/random_token/extra`,
    `${window.location.origin}/s/random.token`
  ])('rejects an invalid OpenCloud link %s without sending it to SimpleDMS', async (webUrl) => {
    const opened = { closed: false, close: vi.fn(), opener: window, location: { replace: vi.fn() } }
    vi.mocked(window.open).mockReturnValue(opened as never)
    createLink.mockResolvedValue(share({ webUrl }))
    await getAction().handler({
      space,
      resources: [resource()]
    } as never)
    expect(opened.close).toHaveBeenCalled()
    expect(opened.location.replace).not.toHaveBeenCalled()
    expect(deletePermission).toHaveBeenCalledWith('drive-1', 'file-123', 'permission-1')
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).toMatch(
      /Could not prepare the file/
    )
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).not.toContain(webUrl)
  })

  it('uses the fallback navigation when popup blocking closes the reserved tab', async () => {
    vi.mocked(window.open).mockReturnValue(null)
    const assign = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined)
    await getAction().handler({
      space,
      resources: [resource()]
    } as never)
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('source=opencloud'))
  })

  it('closes the reserved tab and reports a generic error when link creation fails', async () => {
    const opened = { closed: false, close: vi.fn(), opener: window }
    vi.mocked(window.open).mockReturnValue(opened as never)
    createLink.mockRejectedValue(new Error('upstream-secret-url'))
    await getAction().handler({
      space,
      resources: [resource()]
    } as never)
    expect(opened.close).toHaveBeenCalled()
    expect(showErrorMessage).toHaveBeenCalledWith({
      title: 'Export to SimpleDMS failed',
      errors: [expect.any(Error)]
    })
    expect(showErrorMessage.mock.calls[0][0].errors[0].message).not.toContain('upstream-secret-url')
  })

  it('rechecks visibility in the handler', async () => {
    await getAction().handler({
      space,
      resources: [resource({ canDownload: () => false })]
    } as never)
    expect(createLink).not.toHaveBeenCalled()
    expect(window.open).not.toHaveBeenCalled()
  })

  it('revokes the public link after SimpleDMS reports that the file is staged', async () => {
    const opened = { opener: window, closed: false, close: vi.fn(), location: { replace: vi.fn() } }
    vi.mocked(window.open).mockReturnValue(opened as never)

    await getAction().handler({ space, resources: [resource()] } as never)
    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://simpledms.example.com',
        source: opened as never,
        data: { type: 'simpledms:opencloud-import-staged', permissionId: 'permission-1' }
      })
    )

    expect(deletePermission).toHaveBeenCalledWith('drive-1', 'file-123', 'permission-1')
  })
})
