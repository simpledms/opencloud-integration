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

export const buildSimpleDmsImportUrl = (baseUrl: string, downloadUrl: string): string => {
  const parsedDownloadUrl = new URL(downloadUrl)
  if (!['http:', 'https:'].includes(parsedDownloadUrl.protocol)) {
    throw new Error('OpenCloud returned an unsupported download URL.')
  }

  return `${baseUrl}/open-file/from-url?url=${encodeURIComponent(downloadUrl)}`
}
