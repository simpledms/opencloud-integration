import {
  ActionExtension,
  ApplicationSetupOptions,
  FileAction,
  FileActionOptions,
  useClientService,
  useMessages
} from '@opencloud-eu/web-pkg'
import { computed, unref } from 'vue'
import { useGettext } from 'vue3-gettext'
import { buildSimpleDmsImportUrl, normalizeSimpleDmsBaseUrl } from './simpledms'

export const useExtensions = ({ applicationConfig }: ApplicationSetupOptions) => {
  const { $gettext } = useGettext()
  const clientService = useClientService()
  const { showErrorMessage, showMessage } = useMessages()
  const baseUrl = normalizeSimpleDmsBaseUrl(applicationConfig.simpledmsBaseUrl)

  const action = computed<FileAction>(() => ({
    name: 'upload-to-simpledms',
    icon: 'upload-cloud',
    label: () => $gettext('Upload to SimpleDMS'),
    class: 'oc-files-actions-upload-to-simpledms',
    isVisible: ({ resources }: FileActionOptions) => {
      const resource = resources?.[0]
      return Boolean(
        baseUrl &&
        resources?.length === 1 &&
        resource &&
        !resource.isFolder &&
        resource.canDownload?.() === true
      )
    },
    handler: async ({ space, resources }: FileActionOptions) => {
      const resource = resources?.[0]
      if (!baseUrl || !resource || resources?.length !== 1 || resource.isFolder) {
        return
      }

      try {
        showMessage({ title: $gettext('Preparing upload to SimpleDMS...') })
        const downloadUrl = await clientService.webdav.getFileUrl(space, resource, {
          disposition: 'attachment'
        })
        const target = buildSimpleDmsImportUrl(baseUrl, downloadUrl)
        const openedWindow = window.open(target, '_blank')

        if (openedWindow) {
          openedWindow.opener = null
        } else {
          window.location.assign(target)
        }
      } catch (error) {
        showErrorMessage({
          title: $gettext('Upload to SimpleDMS failed'),
          errors: [error instanceof Error ? error : new Error(String(error))]
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
