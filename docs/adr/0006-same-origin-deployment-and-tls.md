# ADR 0006: Same-origin deployment and verified TLS

**Status:** Accepted

## Context

The Web extension needs an authenticated backend endpoint, while SimpleDMS needs
public access to token downloads. Internal container addresses may differ from
the public origin bound into OpenCloud's signed URLs.

## Decision

Expose the companion through OpenCloud's existing public origin. Add a narrow
regex route to the selected proxy policy (`default` in the lab), ahead of the
built-in `/apps/` prefix route. The route is public; issuance authentication is
enforced by the companion.

Require verified HTTPS upstream, with optional private-CA trust and an internal
dial-address override preserving Host and TLS identity. Permit HTTP origins only
for loopback development. Reject upstream redirects and ignore ambient proxy
environment variables. Run the companion non-root with a read-only filesystem.

## Rationale

Same-origin routing avoids CORS and cross-origin bearer-token configuration.
The regex route takes precedence over OpenCloud's broader `/apps/` route.
Explicit internal routing and CA trust preserve signed URLs without disabling
TLS verification; rejecting redirects prevents credentials reaching another target.

## Consequences

No cross-origin frontend credential exchange or additional public port is needed.
Operators must configure reachable origins and TLS trust. Custom proxy policies
must receive the route explicitly.
