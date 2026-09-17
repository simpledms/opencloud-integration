import {
  ActionExtension,
  ApplicationSetupOptions,
  FileAction,
  FileActionOptions,
  useAuthStore,
  useMessages
} from '@opencloud-eu/web-pkg'
import { computed, unref } from 'vue'
import { useGettext } from 'vue3-gettext'
import { buildSimpleDmsImportUrl, normalizeSimpleDmsBaseUrl } from './simpledms'

export const useExtensions = ({ applicationConfig }: ApplicationSetupOptions) => {
  const { $gettext } = useGettext()
  const authStore = useAuthStore()
  const { showErrorMessage, showMessage } = useMessages()
  const baseUrl = normalizeSimpleDmsBaseUrl(applicationConfig.simpledmsBaseUrl)

  const isVisible = ({ space, resources }: FileActionOptions) => {
    const resource = resources?.[0]
    return Boolean(
      baseUrl &&
      authStore.accessToken &&
      !authStore.publicLinkContextReady &&
      space?.driveType !== 'public' &&
      resources?.length === 1 &&
      resource &&
      (resource.fileId || resource.id) &&
      !resource.isFolder &&
      !resource.isInVault &&
      resource.canDownload?.() === true
    )
  }

  const action = computed<FileAction>(() => ({
    name: 'upload-to-simpledms',
    icon: 'upload-cloud',
    label: () => $gettext('Upload to SimpleDMS'),
    class: 'oc-files-actions-upload-to-simpledms',
    isVisible,
    handler: async ({ space, resources }: FileActionOptions) => {
      const resource = resources?.[0]
      if (!isVisible({ space, resources })) {
        return
      }

      let openedWindow: Window | null = null
      try {
        // Reserve the tab during the click gesture, before the authenticated request.
        openedWindow = window.open('about:blank', '_blank')
        if (openedWindow) {
          openedWindow.opener = null
        }
        showMessage({ title: $gettext('Preparing upload to SimpleDMS...') })
        const response = await fetch('/apps/simpledms_integration/api/create-signed-url', {
          method: 'POST',
          credentials: 'omit',
          redirect: 'error',
          signal: AbortSignal.timeout(35000),
          headers: {
            Authorization: `Bearer ${authStore.accessToken}`,
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ fileId: resource.fileId || resource.id })
        })
        if (!response.ok) {
          throw new Error('Token creation failed')
        }
        const payload = await response.json()
        const download = new URL(payload.downloadUrl)
        if (
          download.origin !== window.location.origin ||
          !/^\/apps\/simpledms_integration\/download\/[a-f0-9]{64}$/.test(download.pathname) ||
          download.username ||
          download.password ||
          download.search ||
          download.hash
        ) {
          throw new Error('Invalid companion response')
        }
        const target = buildSimpleDmsImportUrl(baseUrl, download.href)

        if (openedWindow && !openedWindow.closed) {
          openedWindow.location.replace(target)
        } else {
          window.location.assign(target)
        }
      } catch {
        openedWindow?.close()
        showErrorMessage({
          title: $gettext('Upload to SimpleDMS failed'),
          errors: [
            new Error(
              $gettext(
                'Could not prepare the file. Check your session and file permissions, then try again.'
              )
            )
          ]
        })
      }
    }
  }))

  return computed<ActionExtension[]>(() => [
    {
      id: 'eu.simpledms.opencloud.upload',
      type: 'action',
      extensionPointIds: ['global.files.context-actions'],
      action: unref(action)
    }
  ])
}
