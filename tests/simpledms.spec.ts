import { buildSimpleDmsImportUrl, normalizeSimpleDmsBaseUrl } from '../src/simpledms'

describe('normalizeSimpleDmsBaseUrl', () => {
  it('accepts HTTPS URLs and removes trailing slashes', () => {
    expect(normalizeSimpleDmsBaseUrl(' https://simpledms.example.com/// ')).toBe(
      'https://simpledms.example.com'
    )
  })

  it.each(['http://localhost:8080', 'http://127.0.0.1', 'http://[::1]:8080'])(
    'accepts the local development URL %s',
    (url) => {
      expect(normalizeSimpleDmsBaseUrl(url)).toBe(url)
    }
  )

  it.each([
    '',
    'simpledms.example.com',
    'ftp://simpledms.example.com',
    'http://example.com',
    'https://user:secret@simpledms.example.com',
    'https://simpledms.example.com?tenant=one',
    'https://simpledms.example.com#settings'
  ])('rejects the invalid or insecure URL %s', (url) => {
    expect(normalizeSimpleDmsBaseUrl(url)).toBeNull()
  })
})

describe('buildSimpleDmsImportUrl', () => {
  it('uses the same from-url endpoint as the Nextcloud integration', () => {
    const downloadUrl = 'https://cloud.example.com/data/file.pdf?signature=a+b&expires=123'

    expect(buildSimpleDmsImportUrl('https://simpledms.example.com', downloadUrl)).toBe(
      `https://simpledms.example.com/open-file/from-url?url=${encodeURIComponent(downloadUrl)}`
    )
  })

  it('rejects non-HTTP download URLs', () => {
    expect(() =>
      buildSimpleDmsImportUrl('https://simpledms.example.com', 'file:///tmp/private.pdf')
    ).toThrow('OpenCloud returned an unsupported download URL.')
  })
})
