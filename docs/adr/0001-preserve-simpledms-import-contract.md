# ADR 0001: Preserve the SimpleDMS import contract

**Status:** Accepted

## Context

The original request requires the same SimpleDMS endpoints as the Nextcloud
integration. Both platforms should enter the existing user-confirmed import flow.

## Decision

Open `GET /open-file/from-url?url=<encoded download URL>` in SimpleDMS. Preserve
the companion route names `/apps/simpledms_integration/api/create-signed-url`
and `/apps/simpledms_integration/download/{token}`. OpenCloud issuance uses a
JSON file ID rather than Nextcloud's user-relative path.

Keep the platform implementations separate instead of extracting a shared
JavaScript package for the small amount of overlapping URL logic.

## Rationale

Reusing the existing import flow avoids coordinated SimpleDMS changes and keeps
the user experience consistent. A shared package would add build and release
coupling to the different platform stacks for little reusable code.

## Consequences

SimpleDMS needs no new endpoint. The integration imports one file at a time;
batching, synchronization, metadata mapping, and completion polling are outside
this implementation. The common contract must stay consistent across repositories.
