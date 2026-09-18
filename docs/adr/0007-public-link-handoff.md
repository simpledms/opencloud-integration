# ADR 0007: Protected public-link handoff

**Date:** 2026-09-18

**Status:** accepted

## Context

The original OpenCloud signed URL carried the user's write and delete authority.
A companion service narrowed that credential and provided single-use downloads,
but imposed a separate trusted deployment, proxy route, and runtime state. The
integration should favor fewer moving parts while retaining read-only source
access and strict destination controls.

## Decision

The Web extension creates a password-protected Graph link with `type: view` for
the selected file. It passes the public WebDAV URL to the existing SimpleDMS
`/open-file/from-url` flow with an `opencloud` source marker.

SimpleDMS stores the same password in server configuration and sends it only to
the exact configured OpenCloud origin and public-WebDAV path. It rejects
redirects. After staging, SimpleDMS notifies the originating window and the
extension deletes the Graph permission. OpenCloud's end-of-day link expiration
is the cleanup fallback.

Remove the companion backend, container image, proxy configuration, deployment
guide, and publication workflow.

## Rationale

A view link does not inherit the initiating user's write/delete capability. The
native Graph and public-WebDAV APIs provide authorization and transfer without a
custom intermediary. Strict SimpleDMS validation prevents the shared password
from becoming an arbitrary-fetch credential.

## Consequences

Deployment has only the Web extension and a modified SimpleDMS. The password in
frontend application configuration is inspectable and must be treated as defense
in depth. Links are reusable until best-effort revocation or end-of-day expiry;
the atomic single-use guarantee of the companion is intentionally dropped.

## Related Documents

- [Integration flow](../integration-flow.md)
- [Security notes](../file-handoff-security.md)
- [Alternative approaches](../integration-alternatives.md)
