# SimpleDMS OpenCloud Integration

This OpenCloud Web extension adds an **Export to SimpleDMS** action to the file
context menu. It uses password-protected, read-only OpenCloud public links and
does not require a separately deployed companion service.

## Supported versions

- OpenCloud: 7.2.4 and up
- SimpleDMS: a version containing the OpenCloud public-link importer

## How it works

1. A signed-in user selects one downloadable file.
2. The extension creates an OpenCloud `view` link protected by the configured
   shared password and an end-of-day expiration.
3. The extension opens SimpleDMS's `/open-file/from-url` confirmation page with
   the public WebDAV URL, source marker, callback origin, and permission ID.
4. SimpleDMS accepts only the configured OpenCloud origin and
   `/remote.php/dav/public-files/{token}/{filename}` path, then downloads with
   HTTP Basic username `public` and the configured password.
5. Once SimpleDMS stages the file, it notifies the originating OpenCloud window.
   The extension then deletes the link permission. Expiration is the fallback if
   the callback or deletion fails.

The action is hidden for folders, secure-view files, public-link contexts,
encrypted vault files, and incomplete configuration. See the
[integration flow](docs/integration-flow.md) and
[security notes](docs/file-handoff-security.md) for details.

## Build

```sh
pnpm install
pnpm build
```

The installable Web application is generated in `dist/`.

## Install

1. Generate a strong password for this integration.
2. Configure SimpleDMS with the OpenCloud public origin and password:

```env
SIMPLEDMS_OPENCLOUD_ORIGIN=https://cloud.example.com
SIMPLEDMS_OPENCLOUD_PUBLIC_LINK_PASSWORD=<same-policy-compliant-password>
```

`SIMPLEDMS_OPENCLOUD_ORIGIN` is the destination allowlist for the shared
password, not an optional discovery hint. SimpleDMS sends the password only when
the import URL exactly matches this origin and the expected public-WebDAV path.
If the variable is unset or the origin differs, the OpenCloud import fails before
making a download request. This prevents a caller from using `source=opencloud`
to make SimpleDMS disclose the password to an arbitrary server.

The password must satisfy the OpenCloud deployment's public-link password
policy. Requirements are configurable and can include minimum length,
uppercase and lowercase letters, digits, special characters, and rejection of
commonly used or banned passwords. A value rejected by that policy makes Graph
`createLink` return HTTP 400. Configure the exact same accepted value in
SimpleDMS and the OpenCloud Web application.

3. Configure the OpenCloud Web application in `$OC_CONFIG_DIR/apps.yaml` with
   the same password:

```yaml
simpledms-integration:
  config:
    simpledmsBaseUrl: 'https://simpledms.example.com'
    opencloudPublicLinkPassword: '<same-policy-compliant-password>'
```

4. Copy the contents of `dist/` to
   `$OC_DATA_DIR/web/assets/apps/simpledms-integration` and reload OpenCloud Web.

With `opencloud-compose`, use
`opencloud-compose/config/opencloud/apps/simpledms-integration` for the app and
`opencloud-compose/config/opencloud/apps.yaml` for its configuration.

Both origins must use HTTPS. HTTP is accepted only for loopback development.
The SimpleDMS backend must be able to resolve and reach the configured OpenCloud
public origin with a trusted certificate.

The OpenCloud application configuration is delivered to browser code and must
not be treated as a server-only secret. The password is an additional barrier
for temporary read-only links; security also depends on unguessable share tokens,
strict SimpleDMS URL validation, view-only permissions, expiration, and revocation.

## Development configuration

For local extension development, create `src/config.json` (ignored by Git):

```json
{
  "simpledmsBaseUrl": "http://localhost:8080",
  "opencloudPublicLinkPassword": "development-password"
}
```

## Architecture decisions

See the [ADRs](docs/adr/README.md) for the current decision and superseded
companion design. The [alternative approaches](docs/integration-alternatives.md)
record the main tradeoffs.

## License

Copyright (c) 2026-present Marco Beierer

Licensed under the GNU Affero General Public License, version 3 only. See
`COPYING.md`.
