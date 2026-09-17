# File handoff security audit and remediation

## Audit scope and evidence

The audit examined the original SimpleDMS Web extension and OpenCloud 7.2.4
(upstream tag `v7.2.4`). Live Chromium checks used disposable files in the lab
and a separate HTTP client with no session cookies or Authorization header.
No pre-existing files were modified. The SimpleDMS navigation was intercepted;
the unavailable SimpleDMS service was not part of the live download test.

## Findings in the original implementation

### High: the download credential grants more than read access

The extension passed `webdav.getFileUrl(...)` directly to SimpleDMS. With that
same signed URL, the clean client could GET (200), PUT replacement content
(204), GET the replacement (200), and DELETE the disposable file (204).

OpenCloud's JWT URL authenticator verifies the target URL and signature, then
authenticates as the issuing user without binding the credential to an HTTP
method. The affected code is upstream
`services/proxy/pkg/middleware/signed_url_auth.go`, method `authenticate`, and
`vendor/github.com/opencloud-eu/reva/v2/pkg/signedurl/jwt.go`.
The `attachment` disposition is not an authorization restriction.

### High: download does not consume the credential

Two consecutive unauthenticated GETs returned identical bytes. For normal
authenticated files, the JWT's `exp - iat` was 1,800 seconds. OpenCloud's
`ocdav/propfind/propfind.go`, function `downloadURL`, signs for 30 minutes.
The verifier is stateless: there is no consumed-token record.

Tampered signatures and changed target paths returned 401. An isolated test
of the tagged signer confirmed expiry rejection; live tests did not wait the
full 30 minutes. PROPFIND also returned 207. Renewal beyond the original
expiry was not demonstrated.

### Medium: public shares follow a different expiration policy

The upstream public-share branch can return a passwordless public URL without
a short-lived JWT. The extension did not exclude public-link contexts.
Source inspection confirms this path; live passwordless-share testing was
blocked by the lab's mandatory-password policy.

### Medium: credential exposure in URLs

The whole upstream bearer URL was embedded in the SimpleDMS query string,
exposing it to browser history and any systems retaining full request URLs.
SimpleDMS log retention was not audited. The download response had no
Cache-Control header. Expiry cannot retract bytes already downloaded or cached.

### Build dependencies

The standard pnpm dependency set reported 32 advisories (12 high, 19 moderate,
1 low) through the SDK's module-federation tooling: axios 1.13.5, ws 8.18.0,
and adm-zip 0.5.18. Per the project owner's decision, standard updates are used
without custom overrides. These advisories remain a separate upstream/tooling
concern; passing the application tests does not resolve them.

## Recommended design

Keep the Web action and the SimpleDMS `GET /open-file/from-url?url=...`
contract. Replace the exposed upstream credential with a companion service
maintained in this repository and deployed alongside OpenCloud.

- An authenticated POST accepts an OpenCloud file reference, not an arbitrary
  URL. The service verifies access against the configured OpenCloud origin.
- Issue a cryptographically random opaque token, valid for ten minutes.
- Store only its hash in a bounded in-memory map with the short-lived upstream
  download reference. Keep the upstream credential server-side and purge expired
  entries.
- A public GET atomically deletes/consumes the token before streaming the file.
  Requests with other methods must not download or consume it.
- Stream bytes through the service. Never redirect the recipient to OpenCloud.
- Use HTTPS, no-store responses, bounded requests, and redacted errors. Do not
  follow redirects that could expose credentials or enable arbitrary fetching.
- Only signed-in users and ordinary downloadable files are supported initially;
  public-share export is excluded.

Consumption happens when the first authorized download starts, not when the
transfer finishes. A failed transfer requires a new export. Concurrent requests
must have exactly one winner. A frontend-only app cannot enforce this behavior.

The companion introduces a trusted credential-handling boundary. Authenticate
token issuance and do not expose upstream URLs in responses, logs, or process
memory dumps. Run one service instance. The token map has no persistence: a
restart invalidates all pending exports without affecting source files or
completed imports.

## Verification required for the replacement

- Issuance rejects unauthenticated users, inaccessible resources, folders,
  and arbitrary URLs/unsafe path references.
- Download is GET-only, expires, fails closed on restart, and is atomic
  under concurrent requests. HEAD/PUT/DELETE cannot consume or modify a file.
- A second GET fails; a failed first transfer still consumes the token.
- Upstream credentials, redirects, and raw upstream errors never reach clients.
- Browser action passes only the companion URL to SimpleDMS.

## Implemented remediation and verification

The replacement is implemented in `backend/` with a Go HTTP service and a bounded
mutex-protected token map. The initial implementation used SQLite; the project
owner subsequently chose in-memory storage, accepting lost pending exports on
restart. The backend now uses only Go's standard library and needs no data volume.
The updated Web action calls the companion rather than `webdav.getFileUrl`.
Deployment instructions are in [companion deployment](companion-deployment.md).

The lab runs the companion alongside OpenCloud 7.2.4. OpenCloud's selected
`default` proxy policy uses a regex route before the built-in `/apps/` prefix.
TLS verification from the companion to OpenCloud uses the lab's public
certificate, with the internal dial address overridden without changing Host
or the signed URL. No administrator credential is configured in the companion.

Initial live Chromium and clean-client checks of the companion handoff:

| Check                                     | Result                                                       |
| ----------------------------------------- | ------------------------------------------------------------ |
| Anonymous issuance / invalid bearer token | 401 / 401                                                    |
| Web context action                        | Sends a file ID; opens SimpleDMS with the companion URL only |
| Public HEAD, PUT, DELETE, PROPFIND        | 405; token remains usable                                    |
| First GET / repeated GET                  | 200 / 404                                                    |
| Two concurrent GETs                       | Exactly one 200 and one 404                                  |
| Nonempty file download                    | 61 expected bytes, 61 received, byte-for-byte equal          |
| Filename containing `ä`, space, `#`, `+`  | Preserved in Content-Disposition                             |
| Upstream signed URL in handoff            | Absent; only opaque 64-hex companion token                   |
| Cache policy                              | `no-store`                                                   |

The automated Go suite covers expiry, invalidation when a new store starts,
parallel issuance limits, quota reclamation after expiry/consumption, denied
source permissions, hashed token storage, redirect rejection, and consumed tokens
after upstream failure. The lookup/delete/expiry critical section guarantees one
winner among concurrent downloads. No mutex is held during upstream network I/O.

The SimpleDMS destination was intercepted with harmless HTML for the browser
checks; actual SimpleDMS storage/login was not verified. Live tests did not wait
ten minutes for expiration or simulate a process crash mid-stream. Those
limitations do not replace the automated expiry/restart tests. All modifying
live operations were confined to disposable audit fixtures, which were removed.

### In-memory store follow-up

After replacing SQLite, the Go race suite and vet passed, including parallel
issuance limits and restart invalidation. `go list -m all` lists only the
companion module; there are no third-party Go dependencies.

A live check issued a token, restarted only the companion, then confirmed the
old URL returned 404. A fresh export downloaded 66 bytes with HTTP 200 and
returned 404 on replay. HEAD, PUT, and DELETE returned 405 without consuming
another token, whose subsequent GET still succeeded. The fixture was removed.
Concurrent-download behavior is covered by the in-memory race suite; it was not
repeated in this restart-focused browser check.

The lab companion has only the read-only TLS certificate mount. Its old SQLite
volume was removed; OpenCloud's config and document volumes were preserved.

This fixes the integration boundary. It does not change OpenCloud's native
signed URLs or resolve the separately documented frontend build-tool advisories.
