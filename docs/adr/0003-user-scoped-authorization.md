# ADR 0003: User-scoped authorization and file IDs

**Date:** 2026-09-18

**Status:** superseded

**Superseded by:** [ADR 0007](0007-public-link-handoff.md)

## Context

Export must respect the initiating user's permissions without creating an
administrator-backed download service or an arbitrary-URL fetch proxy.

## Decision

Require an OpenCloud bearer token for issuance. Validate it through
`/graph/v1.0/me`, then request file metadata with an authenticated Depth-0 DAV
PROPFIND. Accept a bounded file ID, not a caller-supplied URL or path.

Reject folders, secure-view resources, and incomplete metadata. Accept upstream
download URLs only from the configured OpenCloud origin and regular-file DAV
endpoint. Do not persist the user's bearer token or trust cookies/user headers
as authentication. Reject cross-origin issuance requests.

## Rationale

Delegating session and permission checks to OpenCloud avoids duplicating its
authorization rules or granting exports administrator privileges. File IDs and
a fixed upstream origin prevent clients from choosing arbitrary fetch targets.

## Consequences

OpenCloud remains the authorization authority. No administrator credential is
needed. Anonymous/public-share exports and arbitrary remote sources are unsupported.
