# Companion deployment

## Components

The Web app is built into `dist/` and installed under the OpenCloud app name
`simpledms-integration`. The Go companion is built by the root `Dockerfile`.
Both live in this repository; deploy both in the same Compose project.

Use [the Compose example](../deploy/compose.example.yaml) and
[proxy routes](../deploy/proxy.yaml). Merge these with existing configuration
rather than replacing other routes or volumes. The example adds a regex route
to the selected `default` policy so it precedes the built-in `/apps/` prefix
route. If your installation selects a custom policy, add it there instead.
The proxy forwards the original
path and Authorization header. Its integration route is public because downloads
use opaque tokens; the companion independently requires OAuth authentication for
POST requests. Do not configure the proxy to log full integration download URLs.

## Configuration

| Variable                        | Purpose/default                                                                                                                   |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `OPENCLOUD_URL`                 | Required public OpenCloud origin, e.g. `https://cloud.example.com`. No path/query/credentials.                                    |
| `INTEGRATION_PUBLIC_ORIGIN`     | Required public origin serving the companion routes. Must equal the Web app origin.                                               |
| `OPENCLOUD_CONNECT_ADDRESS`     | Optional internal `host:port`, such as `opencloud:9200`. Preserves the public HTTP Host, signed URL, and TLS server name.         |
| `OPENCLOUD_CA_FILE`             | Optional PEM CA/certificate file for a private CA or a local self-signed OpenCloud certificate. TLS verification remains enabled. |
| `INTEGRATION_LISTEN_ADDRESS`    | `:8080`; expose only through the trusted HTTPS reverse proxy.                                                                     |
| `INTEGRATION_TOKEN_TTL_SECONDS` | `600`; accepts 1–600 seconds. Shorter values can be used for expiry verification.                                                 |

HTTP origins are accepted only for loopback development. Environment proxy
settings are not used for upstream credential-bearing requests. All upstream
redirects are rejected, including same-origin redirects. Configure canonical
OpenCloud URLs.

The container runs as UID/GID 1000 with a read-only filesystem and no database or
writable data volume. Pending tokens are held in a mutex-protected in-memory map
keyed by token hashes. Expired entries are removed every minute and on issuance.
Each user can have ten pending tokens, with a global limit of 1000
and at most 32 concurrent active API/stream requests. A full pending-token quota
returns 429; wait for expiry or finish existing downloads.

## Endpoints

- `POST /apps/simpledms_integration/api/create-signed-url`
  - `Authorization: Bearer <OpenCloud access token>`
  - `Content-Type: application/json`, body `{"fileId":"<OpenCloud resource ID>"}`
  - Response: `downloadUrl` and `expiresAt` (Unix seconds).
  - The service calls `/graph/v1.0/me` and a Depth-0 DAV PROPFIND to validate the
    user and ordinary-file access. It never stores the user's bearer token.
- `GET /apps/simpledms_integration/download/{token}`
  - No user session required. The token is the capability.
  - GET-only: HEAD, PUT, DELETE, and other methods return 405.
  - Lookup, expiry validation, and deletion under one mutex consume the token
    before source I/O. The mutex is released before streaming.
    Expired, unknown, or used tokens return 404. Failed source downloads return
    a generic 502 and remain consumed. Upstream redirects are not exposed.
- `GET /healthz` on the internal companion port reports process availability.

Responses use `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.
The integration streams an attachment and never forwards upstream cookies,
Location headers, errors, or the upstream signed URL. Incoming Range and
Authorization headers are not forwarded by the public download handler.

## Updates and troubleshooting

Run **one companion instance**. Its memory is not shared with other instances.
Restarting, redeploying, or crashing the companion invalidates all pending tokens;
they return 404 after restart, and users must start a new export. Completed
imports and original OpenCloud files are unaffected. Graceful shutdown allows
active downloads up to 30 seconds to finish; forced termination interrupts them.
An interrupted transfer needs a new export too. Restart always fails closed.

When upgrading from the initial SQLite implementation, remove
`INTEGRATION_DATABASE` and the companion's `/data` volume mount. Recreate the
companion, then remove its unused SQLite volume, which can contain old upstream
credentials. OpenCloud's own config and document-data volumes remain required.

1. Run frontend tests/type checks/build and backend tests, including `go test -race ./...`.
2. Replace the Web app bundle and rebuild the companion image.
3. Recreate both services and reload Web to avoid a cached old action.

- 401/403 on issuance: sign in again and check file permissions.
- 502 on issuance: check the configured origin, TLS trust, Graph API and DAV
  support. Public shares and external download origins are not supported.
- 404 on download: obtain a fresh export; tokens are intentionally single-use.
- An interrupted transfer cannot resume with the same token.
- A network-isolated SimpleDMS container cannot reach `localhost` in OpenCloud's
  public URL: use a hostname and routing reachable by both services.

The companion enforces single use on request start, not on confirmation that
SimpleDMS stored the document. A successful download does not delete the source
file. Upstream OpenCloud signed-URL behavior is unchanged; this integration
prevents exposure of that credential to the recipient.

See [the audit](file-handoff-security.md) for the motivating risks.
