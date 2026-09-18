# Changelog

## Unreleased

- Add the export action to the file sidebar's Actions tab.
- Package the compiled extension as `simpledms-integration.zip` for GitHub releases.
- Require SimpleDMS 1.17.0.
- Replace broad signed download URLs with password-protected, view-only OpenCloud public links.
- Restrict OpenCloud imports in SimpleDMS to the configured origin and public-WebDAV path.
- Revoke link permissions after SimpleDMS stages the file, with end-of-day expiration as fallback.
- Remove the separately deployed companion backend, container image, and proxy configuration.
- Exclude public-link and encrypted-vault contexts from the extension action.

## 1.0.0

- Initial OpenCloud integration with a file context-menu action.
- Handoff to the SimpleDMS `/open-file/from-url` endpoint using OpenCloud pre-authenticated download URLs.
