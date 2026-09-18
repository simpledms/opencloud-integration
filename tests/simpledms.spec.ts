import {
  buildOpenCloudPublicDownloadUrl,
  buildSimpleDmsOpenCloudImportUrl,
  normalizeSimpleDmsBaseUrl
} from '../src/simpledms'

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

describe('OpenCloud public-link handoff', () => {
  it('builds a password-authenticated public WebDAV URL from a same-origin share URL', () => {
    expect(
      buildOpenCloudPublicDownloadUrl(
        'https://cloud.example/s/random_token',
        'report #1.pdf',
        'https://cloud.example'
      )
    ).toBe('https://cloud.example/remote.php/dav/public-files/random_token/report%20%231.pdf')
  })

  it.each([
    'https://evil.example/s/random_token',
    'https://cloud.example/s/random_token/extra',
    'https://cloud.example/s/random.token'
  ])('rejects an invalid OpenCloud share URL %s', (shareUrl) => {
    expect(() =>
      buildOpenCloudPublicDownloadUrl(shareUrl, 'report.pdf', 'https://cloud.example')
    ).toThrow()
  })

  it('marks the SimpleDMS request as an OpenCloud import with cleanup metadata', () => {
    const url = new URL(
      buildSimpleDmsOpenCloudImportUrl(
        'https://simpledms.example',
        'https://cloud.example/remote.php/dav/public-files/token/report.pdf',
        'https://cloud.example',
        'permission-1',
        '/Projects/Quarterly/report.pdf'
      )
    )

    expect(url.pathname).toBe('/open-file/from-url')
    expect(url.searchParams.get('source')).toBe('opencloud')
    expect(url.searchParams.get('callback_origin')).toBe('https://cloud.example')
    expect(url.searchParams.get('permission_id')).toBe('permission-1')
    expect(url.searchParams.get('file_path')).toBe('/Projects/Quarterly/report.pdf')
  })
})
