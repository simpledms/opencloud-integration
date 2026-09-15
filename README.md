# SimpleDMS OpenCloud Integration

This OpenCloud Web extension adds an **Upload to SimpleDMS** action to the context menu for files.

## Supported versions

- OpenCloud: 7.x
- SimpleDMS: 1.9.0 and up

## How it works

1. A user selects **Upload to SimpleDMS** from a file's context menu.
2. The extension asks OpenCloud for the file's short-lived, pre-authenticated download URL.
3. The extension opens SimpleDMS at `GET /open-file/from-url?url=...`, the same SimpleDMS endpoint used by the Nextcloud integration.
4. SimpleDMS downloads the file and continues its open-file flow.

The action is available for one downloadable file at a time. It is hidden for folders, secure-view files, and when the SimpleDMS URL is not configured.

## Build

```sh
pnpm install
pnpm build
```

The installable application is generated in `dist/`.

## Install

1. Copy the contents of `dist/` to `$OC_DATA_DIR/web/assets/apps/simpledms-integration` on the OpenCloud server.
2. Configure the application in `$OC_CONFIG_DIR/apps.yaml`:

```yaml
simpledms-integration:
  config:
    simpledmsBaseUrl: 'https://simpledms.example.com'
```

3. Restart OpenCloud.

With `opencloud-compose`, use `opencloud-compose/config/opencloud/apps/simpledms-integration` for the app and `opencloud-compose/config/opencloud/apps.yaml` for its configuration.

The base URL must use HTTPS. HTTP is accepted only for `localhost`, `127.0.0.1`, and `::1` development instances.

## Operational requirements

- The SimpleDMS backend must be able to reach the OpenCloud URL returned for the selected file.
- OpenCloud controls the lifetime of its pre-authenticated download URLs. The SimpleDMS import should begin immediately after the action is selected.
- Opening SimpleDMS is a top-level navigation and does not require a `connect-src` CSP exception.

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
