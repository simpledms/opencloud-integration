# ADR 0004: Read-only, expiring, single-use tokens

**Date:** 2026-09-18

**Status:** superseded

**Superseded by:** [ADR 0007](0007-public-link-handoff.md)

## Context

SimpleDMS needs temporary file access, but recipients must not modify the source
or reuse the download capability.

## Decision

Issue 256-bit random opaque tokens, indexed server-side by SHA-256 hash. Expire
them after ten minutes; administrators may configure a shorter lifetime of
1–600 seconds.

Allow only GET downloads. Atomically validate and remove a token before upstream
I/O. Reject other methods without consuming it. Return 404 for expired, unknown,
used, or lost tokens. Upstream failures still leave the token consumed.

## Rationale

Consuming before I/O closes the replay window that would exist while waiting for
transfer completion. Opaque tokens hide the upstream credential. Ten minutes
matches the Nextcloud flow and gives users time to confirm while bounding exposure.

## Consequences

Concurrent requests have one winner. Failed transfers and interrupted downloads
require a new export; resuming with the same token is unsupported. Consumption
means download start, not confirmation that SimpleDMS stored the document.
Already downloaded bytes cannot be revoked.
