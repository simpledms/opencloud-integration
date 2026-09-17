# ADR 0005: Bounded in-memory tokens and one instance

**Status:** Accepted

## Context

Tokens represent short-lived access, not durable documents. The initial SQLite
implementation preserved pending exports across restarts; the project owner
accepted losing them to simplify deployment.

## Decision

Keep pending token hashes and upstream references in a mutex-protected map.
Perform quota checks, insertion, lookup, expiry validation, and removal under
the lock; release it before streaming. Limit pending tokens to ten per user and
1,000 globally, and active API/stream requests to 32. Purge expired entries every
minute and during issuance.

Run one companion instance with no writable data volume. Do not use in-memory
SQLite, filesystem-backed tokens, or a shared store.

## Rationale

A map and mutex provide the required atomicity without database operations or
filesystem recovery logic. In-memory SQLite would retain an unnecessary dependency.
Bounds limit resource use; one instance avoids distributed coordination. Retrying
a pending export after restart is an acceptable cost for this temporary workflow.

## Consequences

Restarts, crashes, and redeployments invalidate pending exports, failing closed.
Users must export again. Completed imports and source files are unaffected.
Multiple independent instances would not share tokens and are unsupported.
