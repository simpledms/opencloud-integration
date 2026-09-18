import {
  ActionExtension,
  ApplicationSetupOptions,
  FileAction,
  FileActionOptions,
  useAuthStore,
  useClientService,
  useMessages
} from '@opencloud-eu/web-pkg'
import { computed, unref } from 'vue'
import { useGettext } from 'vue3-gettext'
import {
  buildOpenCloudPublicDownloadUrl,
  buildSimpleDmsOpenCloudImportUrl,
  normalizeSimpleDmsBaseUrl
} from './simpledms'

const IMPORT_STAGED_MESSAGE = 'simpledms:opencloud-import-staged'
const LOG_PREFIX = '[OpenCloud import]'

// HTTP errors can contain credentials and public URLs in their request config.
const requestFailureDetails = (error: unknown) => {
  const failure = error as {
    code?: string
    response?: { status?: number; headers?: Record<string, string> }
  } | null
  return {
    errorName: error instanceof Error ? error.name : typeof error,
    errorCode: failure?.code ?? null,
    httpStatus: failure?.response?.status ?? null,
    requestId: failure?.response?.headers?.['x-request-id'] ?? null
  }
}

export const useExtensions = ({ applicationConfig }: ApplicationSetupOptions) => {
  const { $gettext } = useGettext()
  const authStore = useAuthStore()
  const clientService = useClientService()
  const { showErrorMessage, showMessage } = useMessages()
  const baseUrl = normalizeSimpleDmsBaseUrl(applicationConfig.simpledmsBaseUrl)
  const publicLinkPassword =
    typeof applicationConfig.opencloudPublicLinkPassword === 'string'
      ? applicationConfig.opencloudPublicLinkPassword
      : ''

  const isVisible = ({ space, resources }: FileActionOptions) => {
    const resource = resources?.[0]
    return Boolean(
      baseUrl &&
      publicLinkPassword &&
      authStore.accessToken &&
      !authStore.publicLinkContextReady &&
      space?.driveType !== 'public' &&
      resources?.length === 1 &&
      space?.id &&
      resource &&
      resource.id &&
      resource.name &&
      !resource.isFolder &&
      !resource.isInVault &&
      resource.canDownload?.() === true
    )
  }

  const action = computed<FileAction>(() => ({
    name: 'upload-to-simpledms',
    icon: 'upload-cloud',
    label: () => $gettext('Export to SimpleDMS'),
    class: 'oc-files-actions-upload-to-simpledms',
    isVisible,
    handler: async ({ space, resources }: FileActionOptions) => {
      const resource = resources?.[0]
      if (!isVisible({ space, resources })) {
        return
      }

      let openedWindow: Window | null = null
      let permissionId = ''
      const revokePermission = async (reason: string): Promise<void> => {
        if (!permissionId) {
          return
        }
        try {
          await clientService.graphAuthenticated.permissions.deletePermission(
            space.id,
            resource.id,
            permissionId
          )
        } catch (error) {
          console.error(`${LOG_PREFIX} share deletion failed`, {
            permissionId,
            reason,
            ...requestFailureDetails(error)
          })
          // Expiration remains the fallback when immediate cleanup fails.
        }
      }
      try {
        // Reserve the tab during the click gesture, before the authenticated request.
        openedWindow = window.open('about:blank', '_blank')
        if (openedWindow) {
          openedWindow.opener = window
        }
        showMessage({ title: $gettext('Preparing export to SimpleDMS...') })
        const link = await clientService.graphAuthenticated.permissions.createLink(
          space.id,
          resource.id,
          {
            type: 'view',
            password: publicLinkPassword,
            displayName: $gettext('SimpleDMS export'),
            expirationDateTime: new Date(Date.now() + 15 * 60 * 1000).toISOString()
          }
        )
        permissionId = link.id
        const downloadUrl = buildOpenCloudPublicDownloadUrl(
          link.webUrl,
          resource.name,
          window.location.origin
        )
        const target = buildSimpleDmsOpenCloudImportUrl(
          baseUrl,
          downloadUrl,
          window.location.origin,
          permissionId,
          resource.path
        )

        if (openedWindow) {
          const simpleDmsOrigin = new URL(baseUrl).origin
          const onImportStaged = (event: MessageEvent) => {
            if (
              event.origin !== simpleDmsOrigin ||
              event.source !== openedWindow ||
              event.data?.type !== IMPORT_STAGED_MESSAGE ||
              event.data?.permissionId !== permissionId
            ) {
              return
            }

            window.removeEventListener('message', onImportStaged)
            void revokePermission('import-staged')
          }
          window.addEventListener('message', onImportStaged)
          window.setTimeout(
            () => window.removeEventListener('message', onImportStaged),
            26 * 60 * 60 * 1000
          )
        }

        if (openedWindow && !openedWindow.closed) {
          openedWindow.location.replace(target)
        } else {
          window.location.assign(target)
        }
      } catch (error) {
        console.error(`${LOG_PREFIX} export preparation failed`, {
          permissionId,
          ...requestFailureDetails(error)
        })
        void revokePermission('export-failed')
        openedWindow?.close()
        showErrorMessage({
          title: $gettext('Export to SimpleDMS failed'),
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
      extensionPointIds: ['global.files.context-actions', 'app.files.sidebar.actions'],
      action: unref(action)
    }
  ])
}
