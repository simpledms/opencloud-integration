const LOCAL_HOSTS = new Set(['localhost', '127.0.0.1', '::1', '[::1]'])

export const normalizeSimpleDmsBaseUrl = (value: unknown): string | null => {
  if (typeof value !== 'string') {
    return null
  }

  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) {
    return null
  }

  try {
    const url = new URL(trimmed)
    if (url.username || url.password || url.search || url.hash) {
      return null
    }
    if (url.protocol === 'https:') {
      return trimmed
    }
    if (url.protocol === 'http:' && LOCAL_HOSTS.has(url.hostname.toLowerCase())) {
      return trimmed
    }
  } catch {
    return null
  }

  return null
}

export const buildOpenCloudPublicDownloadUrl = (
  shareUrl: string,
  fileName: string,
  openCloudOrigin: string
): string => {
  const share = new URL(shareUrl)
  const origin = new URL(openCloudOrigin)
  const match = share.pathname.match(/^\/s\/([^/]+)\/?$/)

  if (
    share.origin !== origin.origin ||
    share.username ||
    share.password ||
    share.search ||
    share.hash ||
    !match ||
    !fileName
  ) {
    throw new Error('OpenCloud returned an invalid public link.')
  }

  const token = decodeURIComponent(match[1])
  if (!/^[A-Za-z0-9_-]+$/.test(token)) {
    throw new Error('OpenCloud returned an invalid public link token.')
  }

  return `${origin.origin}/remote.php/dav/public-files/${encodeURIComponent(token)}/${encodeURIComponent(fileName)}`
}

export const buildSimpleDmsOpenCloudImportUrl = (
  baseUrl: string,
  downloadUrl: string,
  callbackOrigin: string,
  permissionId: string,
  filePath: string
): string => {
  const target = new URL(`${baseUrl}/open-file/from-url`)
  target.searchParams.set('url', downloadUrl)
  target.searchParams.set('source', 'opencloud')
  target.searchParams.set('callback_origin', callbackOrigin)
  target.searchParams.set('permission_id', permissionId)
  if (filePath) {
    target.searchParams.set('file_path', filePath)
  }
  return target.href
}
