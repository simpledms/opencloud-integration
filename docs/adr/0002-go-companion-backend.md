# ADR 0002: Same-repository Go companion backend

**Status:** Accepted

## Context

Browser code cannot enforce server-to-server token consumption or keep a
forwarded upstream credential private. OpenCloud Web apps do not execute custom
backend handlers from their app directory.

The initial direct-URL handoff exposed an OpenCloud credential that remained
reusable after download and permitted overwriting/deleting the target file.
See the [audit](../file-handoff-security.md) for evidence.

## Decision

Maintain a small Go HTTP service in `backend/` alongside the Web extension.
Deploy it as a separate container. Keep upstream signed URLs server-side and
stream file bytes through the companion, never redirecting recipients upstream.

Use attachment responses, `no-store`, `no-referrer`, and generic errors. Do not
forward incoming download credentials, cookies, Range headers, or upstream
Location/Set-Cookie headers. Do not persist file content.

## Rationale

A server-side intermediary can enforce single use without exposing OpenCloud's
broader credential. Go provides streaming HTTP and synchronization in one small
binary. Keeping both components together makes their API and deployment changes
easier to maintain without modifying OpenCloud itself.

## Consequences

The companion becomes a trusted credential-handling boundary and must be
installed with the Web app. It needs no OpenCloud core changes. The current
backend uses only Go's standard library.
