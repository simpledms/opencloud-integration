# Architecture decision records

The architectural choices worth preserving: integration boundaries, security
guarantees, storage tradeoffs, and deployment constraints. Relevant rejected
approaches are summarized within each decision.

- [0001: Preserve the SimpleDMS import contract](0001-preserve-simpledms-import-contract.md)
- [0002: Same-repository Go companion backend](0002-go-companion-backend.md)
- [0003: User-scoped authorization and file IDs](0003-user-scoped-authorization.md)
- [0004: Read-only, expiring, single-use tokens](0004-read-only-single-use-tokens.md)
- [0005: Bounded in-memory tokens and one instance](0005-bounded-in-memory-token-storage.md)
- [0006: Same-origin deployment and verified TLS](0006-same-origin-deployment-and-tls.md)

Supporting documents:

- [Integration flow and OpenCloud API requests](../integration-flow.md)
- [Audit findings and evidence](../file-handoff-security.md)
- [Deployment guide](../companion-deployment.md)
