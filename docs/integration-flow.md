# OpenCloud-SimpleDMS integration flow

OpenCloud remains responsible for user authorization, source storage, and link
permissions. SimpleDMS performs the server-to-server download from OpenCloud's
public WebDAV endpoint. No integration-specific backend runs between them.

```text
OpenCloud Web extension
    | 1. Authenticated Graph createLink(type=view, password, expiration)
    v
OpenCloud
    | 2. Return permission ID and /s/{token} URL
    v
OpenCloud Web extension
    | 3. Open SimpleDMS /open-file/from-url with public DAV URL
    v
SimpleDMS
    | 4. Validate exact OpenCloud origin/path and GET with shared password
    v
OpenCloud public WebDAV
    | 5. Stream file bytes
    v
SimpleDMS stages the file
    | 6. postMessage(permission ID) to the originating window
    v
OpenCloud Web extension deletes the permission
```

## 1. Create the temporary view link

The extension action is available in the file context menu and the sidebar's
**Actions** tab. It is available only to an authenticated user selecting one
downloadable, non-vault file outside a public-link context. Both entry points call
the authenticated Graph client for the selected drive and item:

```text
createLink({
  type: "view",
  password: <configured shared password>,
  displayName: "SimpleDMS export",
  expirationDateTime: <near-future timestamp>
})
```

OpenCloud applies view-only permissions. Its public-link expiration has day
granularity, so the supplied timestamp becomes an end-of-day fallback rather
than a precise fifteen-minute lifetime.

The extension accepts only a returned URL on its own origin with the exact
`/s/{token}` shape. It converts that URL to:

```text
https://<opencloud-origin>/remote.php/dav/public-files/<token>/<filename>
```

## 2. Open the SimpleDMS confirmation page

The extension opens:

```text
https://<simpledms-origin>/open-file/from-url
  ?url=<encoded-public-WebDAV-URL>
  &source=opencloud
  &callback_origin=<OpenCloud-origin>
  &permission_id=<Graph-permission-ID>
  &file_path=<display-only-OpenCloud-path>
```

The password is not included in this URL. The share token and permission ID can
appear in browser history and HTTP request logs and must still be treated as
temporary capabilities. The selected file path is included only so SimpleDMS
can show the filename and OpenCloud location on the confirmation screen; it is
never used to select or authorize the download.

## 3. Validate and stage in SimpleDMS

SimpleDMS applies source-specific checks before displaying the confirmation page
and repeats URL validation before downloading:

- the origin must exactly match `SIMPLEDMS_OPENCLOUD_ORIGIN`;
- the URL must have no credentials, query, or fragment;
- the path must match
  `/remote.php/dav/public-files/{token}/{nonempty-file-path}`;
- the token may contain only letters, digits, `_`, and `-`;
- redirects and ambient HTTP proxy settings are disabled for this source; and
- resolved loopback addresses are rejected unless SimpleDMS runs with `-dev`;
  private, link-local, multicast, and configured metadata addresses remain blocked.

HTTPS downloads use normal TLS verification except for the
[development-mode loopback TLS exception](file-handoff-security.md#development-mode-loopback-tls).

SimpleDMS then sends a GET with `Authorization: Basic` for username `public` and
`SIMPLEDMS_OPENCLOUD_PUBLIC_LINK_PASSWORD`. OpenCloud independently validates
the token, password, expiry, and view permission before returning bytes.

## 4. Revoke after staging

After SimpleDMS has staged the temporary upload, the confirmation window sends a
`simpledms:opencloud-import-staged` message containing the permission ID to its
opener. Both sides validate the configured origin. The extension additionally
checks the message source and expected permission ID before calling Graph
`deletePermission` for the selected drive and item.

Revocation is best effort. A browser crash, blocked popup, same-tab fallback,
lost callback, or failed Graph deletion can leave the view link usable until
OpenCloud's end-of-day expiration. Already downloaded bytes cannot be revoked.

## Related documents and code

- [Security findings and current controls](file-handoff-security.md)
- [Alternative approaches and tradeoffs](integration-alternatives.md)
- [Architecture decisions](adr/README.md)
- [Web action](../src/useExtensions.ts)
- [URL construction](../src/simpledms.ts)
