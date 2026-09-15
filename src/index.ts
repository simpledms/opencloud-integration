import { defineWebApplication } from '@opencloud-eu/web-pkg'
import { useGettext } from 'vue3-gettext'
import translations from '../l10n/translations.json'
import { useExtensions } from './useExtensions'

export default defineWebApplication({
  setup(args) {
    const { $gettext } = useGettext()

    return {
      appInfo: {
        id: 'simpledms-integration',
        name: $gettext('SimpleDMS integration')
      },
      translations,
      extensions: useExtensions(args)
    }
  }
})
