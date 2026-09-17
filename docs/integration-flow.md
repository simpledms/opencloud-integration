# OpenCloud–SimpleDMS integration flow

The integration has two components: a Web extension loaded by OpenCloud and a
companion backend running alongside it. OpenCloud remains responsible for users,
file storage, and permissions. The companion provides restricted, one-time
download access for SimpleDMS.

```text
OpenCloud Web extension
    │ 1. File ID + user's access token
    ▼
Companion backend ── 2. Validate user and resolve file ──► OpenCloud
    │
    └── 3. Return a one-time companion URL to the extension
                         │
                         └── Open SimpleDMS import in the browser
                                      │
                                      │ 4. GET one-time URL after confirmation
                                      ▼
                               Companion backend ── GET source file ──► OpenCloud
                                      │
                                      └── Stream file bytes ──► SimpleDMS
```

## 1. The user starts an export

The user selects **Upload to SimpleDMS** in the OpenCloud file context menu.
The extension sends the selected file's ID and the user's current OAuth access
token to the companion:

```http
POST /apps/simpledms_integration/api/create-signed-url
Authorization: Bearer <user-access-token>
Content-Type: application/json

{"fileId":"<OpenCloud-resource-ID>"}
```

The request uses OpenCloud's existing HTTPS origin. OpenCloud's proxy routes
the integration path to the companion, which independently authenticates
issuance. The caller supplies a file ID, not an arbitrary download URL.

## 2. The companion checks access with OpenCloud

The companion uses ordinary HTTPS requests to OpenCloud's Graph and WebDAV APIs.
Both requests carry the user's bearer token. It needs no administrator account,
direct filesystem access, database access, or private RPC interface.

### Validate the user through the Graph API

```http
GET /graph/v1.0/me
Authorization: Bearer <user-access-token>
```

OpenCloud validates the access token and returns the user's identity. The
companion requires a successful response containing a user ID, which it also
uses to enforce the per-user pending-token limit.

### Resolve the selected file through WebDAV

```http
PROPFIND /remote.php/dav/spaces/{fileId}
Authorization: Bearer <user-access-token>
Depth: 0
Content-Type: application/xml

<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:prop>
    <d:resourcetype/>
    <d:getcontentlength/>
    <oc:permissions/>
    <oc:downloadURL/>
  </d:prop>
</d:propfind>
```

`{fileId}` is validated and URL-escaped by the companion. `Depth: 0` requests
only the selected resource, not a directory listing. OpenCloud authorizes the
request using the user's permissions.

The companion expects a `207 Multi-Status` response and reads successful
properties for exactly one resource:

| Property               | Use                                                     |
| ---------------------- | ------------------------------------------------------- |
| `DAV:resourcetype`     | Reject folders.                                         |
| `DAV:getcontentlength` | Require valid, nonnegative file-size metadata.          |
| `oc:permissions`       | Reject secure-view files marked with `X`.               |
| `oc:downloadURL`       | Obtain OpenCloud's signed URL for the later source GET. |

Missing required metadata is rejected. The download URL must use the configured
OpenCloud origin and regular-file DAV endpoint, with a signed-URL parameter.
Public-share URLs and other origins are rejected; redirects are not followed.

**The upstream signed URL stays inside the companion.** The user's access token
is used only for these authorization requests and is not retained with the export.
The implementation is in [`userID()` and `file()`](../backend/opencloud.go).

## 3. The companion returns a one-time URL

After authorization, the companion generates a random token. It keeps the token
hash, owner, expiry, filename, and upstream download reference in bounded memory.
It returns `downloadUrl` and `expiresAt` to the extension. The URL points to:

```text
https://<opencloud-origin>/apps/simpledms_integration/download/<opaque-token>
```

The extension validates this response and opens SimpleDMS at:

```text
https://<simpledms-origin>/open-file/from-url?url=<encoded-companion-URL>
```

This is the same SimpleDMS entrypoint used by the Nextcloud integration.

## 4. SimpleDMS downloads through the companion

After the user confirms the URL, the SimpleDMS backend fetches it without an
OpenCloud session. The opaque token grants access to that download.

The companion atomically checks expiry and consumes the token, then releases
the store lock. It performs a GET against the private upstream signed URL and
streams the response to SimpleDMS as a non-cacheable attachment. It does not
redirect SimpleDMS to OpenCloud or expose the upstream credential. OpenCloud
handles authorization of the source GET too.

Tokens last at most ten minutes and are consumed when downloading starts, even
if the transfer subsequently fails. Reuse returns 404; write methods are rejected.
The source file is not deleted or modified.

Restarting the companion clears pending tokens, requiring a new export. Completed
imports and original OpenCloud files remain unaffected.

## Related documents and code

- [Deployment and configuration](companion-deployment.md)
- [Security findings and verification](file-handoff-security.md)
- [Architecture decisions](adr/README.md)
- [Web action](../src/useExtensions.ts)
- [Companion HTTP handlers](../backend/server.go)
- [In-memory token store](../backend/store.go)
