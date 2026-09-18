# File handoff security

## Original signed-URL findings

The original extension passed `webdav.getFileUrl(...)` directly to SimpleDMS.
An audit against OpenCloud 7.2.4 found that the resulting signed URL was broader
than a download capability:

- the same unauthenticated URL allowed GET, PUT replacement, and DELETE;
- two consecutive GET requests returned the file, so download did not consume it;
- normal authenticated-file URLs remained valid for about 30 minutes; and
- the credential appeared in the SimpleDMS navigation query and potentially in
  browser history and full-URL logs.

OpenCloud's signed-URL authenticator verifies the target URL and signature, then
authenticates as the issuing user without binding the credential to one HTTP
method. Discarding the URL after GET does not revoke it.

## Current design

The extension no longer hands an authenticated-user signed URL to SimpleDMS. It
creates a dedicated Graph link with `type: view`, a configured password, and an
expiration, then passes only that link's public WebDAV token URL.

SimpleDMS has an explicit `opencloud` source mode. It accepts only the configured
OpenCloud origin and public-WebDAV path, supplies the password server-side, does
not follow redirects, bypasses ambient proxies, and rejects unsafe resolved
addresses. Generic callers cannot choose where the OpenCloud password is sent.

`SIMPLEDMS_OPENCLOUD_ORIGIN` is therefore part of the password-protection
boundary. It is not inferred from the caller-supplied file URL: doing so would
allow an attacker to submit their own host and receive the shared password in an
HTTP Basic authorization header. OpenCloud imports fail closed when the variable
is unset or does not exactly match the URL origin.

After staging succeeds, SimpleDMS sends the expected permission ID to the exact
OpenCloud callback origin. The extension checks the message origin, source
window, type, and permission ID before deleting that permission through the
authenticated Graph client.

## Security properties

- The source permission is view-only instead of inheriting the user's write and
  delete authority.
- The password is not embedded in the handoff URL.
- SimpleDMS sends the password only to its configured OpenCloud origin and only
  for the public-WebDAV route.
- OpenCloud still validates the share token, password, permission, and expiry.
- Redirect rejection prevents forwarding Basic credentials to another target.
- The source file is not modified or deleted by the integration.
- No integration-specific proxy or credential-holding companion is deployed.

## Deliberate limitations

The OpenCloud Web application configuration is delivered to the browser. The
shared password is therefore inspectable and is defense in depth, not a
server-only secret. An attacker still needs a valid unguessable share token, but
operators must not reuse this password for accounts or unrelated services.

Public links are not single-use. Revocation happens only after SimpleDMS stages
the file and is best effort. Browser crashes, popup blocking, callback loss, or a
failed Graph request can leave the link usable until expiration. OpenCloud rounds
link expiration to end of day, so this fallback can last longer than the intended
interactive handoff. Already transferred bytes cannot be revoked.

The share token, permission ID, and display-only source path travel in a URL and
can appear in browser history or full-URL logs. They should not be logged
deliberately. SimpleDMS uses
generic client-facing download errors. Its server logs distinguish configuration,
TLS/network, authentication, authorization, expiry, redirect, and rate-limit
failures without logging the public-link URL, token, password, or authorization
header.

This design trusts SimpleDMS with the downloaded document and trusts its backend
to protect the configured shared password. It does not turn OpenCloud public
links into one-request capabilities.

## Operational requirements

- Use HTTPS for both public origins and normal certificate verification.
- Set exactly the same integration password in OpenCloud and SimpleDMS. It must
  satisfy the active OpenCloud public-link password policy, including any
  configured length, character-class, and banned-password requirements;
  otherwise Graph link creation fails with HTTP 400.
- Restrict access to OpenCloud's application configuration as for other deployed
  frontend assets, while assuming authenticated users can inspect it.
- Ensure SimpleDMS reaches the configured public OpenCloud origin directly.
- Rotate the integration password in both systems if it is disclosed beyond the
  expected browser exposure.

Frontend dependency advisories remain a separate SDK/tooling concern; passing
the application tests does not resolve them.
