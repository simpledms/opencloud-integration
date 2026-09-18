# Alternative integration approaches

This records the main alternatives considered for moving one OpenCloud file into
SimpleDMS. The current design uses a password-protected, view-only public link.

## Comparison

| Approach | Extra service | Credential received by SimpleDMS | Main tradeoff |
| --- | --- | --- | --- |
| Current protected view link | No | Public share token; password stays in SimpleDMS config | Simple deployment and read-only access, but not single-use |
| Single-use companion | Yes | Opaque companion token | Strong one-request semantics, but another trusted service |
| Native signed URL | No | User-authority signed URL | Simple, but audited URL allowed write/delete and replay |
| OAuth connector | No | OAuth tokens | Standard integration, but registration and token lifecycle |
| Browser transfer | No | File bytes only | Narrow credential boundary, but browser carries the transfer |
| WOPI | Collaboration stack | WOPI session token | Native app integration, but much heavier than one import |

## Protected view link

The extension creates a Graph `view` permission with a password and expiration.
SimpleDMS downloads from public WebDAV and the extension revokes the permission
after staging.

Advantages:

- No additional runtime service or proxy route.
- Server-to-server transfer without buffering the file in the browser.
- Permission is scoped to viewing the selected item rather than the user's full
  signed-URL authority.
- SimpleDMS can narrowly constrain where it sends the shared password.

Drawbacks:

- The frontend configuration exposes the shared password to browser users.
- The token is reusable until best-effort revocation or end-of-day expiration.
- A callback requires the OpenCloud tab to remain available; same-tab fallback
  relies on expiration.

## Single-use companion

A companion can accept an authenticated file ID, keep the upstream credential
server-side, and issue an opaque token consumed by the first GET.

This gives strong atomic single-use semantics and hides upstream references, but
adds a trusted service, image lifecycle, proxy routing, limits, and failure state.
That design was implemented previously and is superseded by the deployment
simplification in [ADR 0007](adr/0007-public-link-handoff.md).

## Native signed URL

Passing OpenCloud's regular signed download URL preserves the existing
SimpleDMS endpoint with minimal code. The audited OpenCloud 7.2.4 URL, however,
supported repeated GET plus PUT and DELETE until expiry. It is not suitable as a
read-only handoff credential.

## OAuth connector

SimpleDMS could become a first-class OpenCloud OAuth client and retrieve files
through Graph or WebDAV. This avoids public links but requires client
registration, callback and consent behavior, scope validation, token storage,
refresh, and revocation. An OAuth token is not automatically restricted to one
file or to read-only use.

## Browser transfer

The extension could download through the user's session and upload the bytes to
SimpleDMS, either after completion or with request streaming. Source credentials
would stay in OpenCloud, but the browser must remain open and carry both network
legs. Large-file buffering, cross-origin authentication, CORS, streaming request
support, proxy buffering, and retry behavior would need dedicated work.

## WOPI

SimpleDMS could implement WOPI discovery and file-session behavior using
OpenCloud's collaboration infrastructure. This is appropriate for broader
application integration, but WOPI sessions are not one-shot download tokens and
the protocol is disproportionate for a single document import.

## Decision

The protected view link is the smallest design that removes the dangerous
user-authority signed URL and the separately deployed companion. Choose a
companion again only if strict atomic single-use behavior becomes more important
than deployment simplicity.
