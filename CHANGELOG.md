# Changelog

## Unreleased

- Replace direct OpenCloud signed-URL sharing with a required companion backend.
- Add read-only, ten-minute, single-use downloads with atomic in-memory consumption.
- Use a bounded memory store with no database dependency or data volume; companion restarts invalidate pending exports.
- Exclude public-link and encrypted-vault exports; keep upstream credentials private.
- Document the security audit, companion configuration, and deployment process.

## 1.0.0

- Initial OpenCloud integration with a file context-menu action.
- Handoff to the SimpleDMS `/open-file/from-url` endpoint using OpenCloud pre-authenticated download URLs.
