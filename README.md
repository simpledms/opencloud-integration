# SimpleDMS OpenCloud Integration

This OpenCloud Web extension and companion backend add an **Upload to SimpleDMS** action to the context menu for files.

## Supported versions

- OpenCloud: 7.x
- SimpleDMS: 1.9.0 and up

## How it works

1. A user selects **Upload to SimpleDMS** from a file's context menu.
2. The extension sends the selected file ID to the companion using the user's OpenCloud bearer token. The companion validates the session and file access with OpenCloud.
3. The extension opens SimpleDMS at `GET /open-file/from-url?url=...`, the same SimpleDMS endpoint used by the Nextcloud integration.
4. The user confirms the URL in SimpleDMS.
5. The companion consumes the one-time token and streams the file to SimpleDMS, which continues the normal open-file flow. OpenCloud's upstream credential is never sent to SimpleDMS.

The action is available for one downloadable file at a time. It is hidden for folders, secure-view files, public links, encrypted vault files, and when the SimpleDMS URL is not configured.

Tokens expire after ten minutes and are consumed when the first GET starts. A second GET fails, including after a failed or interrupted first transfer. Start a new export to retry. HEAD, PUT, and DELETE are rejected without consuming the token. Already downloaded bytes cannot be revoked.

## Build

```sh
pnpm install
pnpm build
```

The installable Web application is generated in `dist/`. Build the companion image from this repository:

```sh
docker build -t simpledms-opencloud-integration .
```

Local backend development requires Go 1.26 or newer. Run `go test ./...` from `backend/`.

## Install

1. Copy the contents of `dist/` to `$OC_DATA_DIR/web/assets/apps/simpledms-integration` on the OpenCloud server.
2. Configure the application in `$OC_CONFIG_DIR/apps.yaml`:

```yaml
simpledms-integration:
  config:
    simpledmsBaseUrl: 'https://simpledms.example.com'
```

3. Deploy the companion backend alongside OpenCloud. Merge [deploy/compose.example.yaml](deploy/compose.example.yaml) into your Compose project and [deploy/proxy.yaml](deploy/proxy.yaml) into OpenCloud's `/etc/opencloud/proxy.yaml`. Set `OPENCLOUD_URL` and `INTEGRATION_PUBLIC_ORIGIN` to the public OpenCloud origin. The companion requires no database or writable data volume.
4. Recreate the services with `docker compose up -d --build` and reload OpenCloud Web.

The companion is required: copying only the Web app is insufficient. See the [deployment guide](docs/companion-deployment.md) for TLS, internal connections, endpoints, and maintenance.

With `opencloud-compose`, use `opencloud-compose/config/opencloud/apps/simpledms-integration` for the app and `opencloud-compose/config/opencloud/apps.yaml` for its configuration.

The base URL must use HTTPS. HTTP is accepted only for `localhost`, `127.0.0.1`, and `::1` development instances.

## Operational requirements

- The SimpleDMS backend must be able to reach and trust the HTTPS certificate of OpenCloud's public origin, where the companion download route is exposed.
- The companion must reach and trust OpenCloud. It has no administrator credentials and authorizes each export with the requesting user's access token.
- Pending token hashes and upstream credentials are held only in bounded process memory. Do not log full token URLs or expose process-memory dumps.
- Run one companion instance. Tokens expire even if the browser closes. Restarting or redeploying the companion invalidates all pending tokens: users must start a new export. Completed imports and original OpenCloud files are unaffected.
- Opening SimpleDMS is a top-level navigation and does not require a `connect-src` CSP exception.

The original direct-URL implementation had read/write and replay risks. See the [security audit and remediation](docs/file-handoff-security.md).

## Architecture decisions

See the [short ADRs](docs/adr/README.md) for architectural decisions and their tradeoffs.

## Development configuration

For local extension development, create `src/config.json` (ignored by Git):

```json
{
  "simpledmsBaseUrl": "http://localhost:8080"
}
```

## License

Copyright (c) 2026-present Marco Beierer

Licensed under the GNU Affero General Public License, version 3 only. See `COPYING.md`.
