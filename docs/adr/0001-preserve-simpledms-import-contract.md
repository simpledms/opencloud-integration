# ADR 0001: Preserve the SimpleDMS import contract

**Date:** 2026-09-18

**Status:** accepted

## Context

The original request requires the same SimpleDMS endpoints as the Nextcloud
integration. Both platforms should enter the existing user-confirmed import flow.

## Decision

Open `GET /open-file/from-url?url=<encoded download URL>` in SimpleDMS. Add an
`opencloud` source marker and callback metadata while retaining the existing
confirmation and temporary-file staging flow.

Keep the platform implementations separate instead of extracting a shared
JavaScript package for the small amount of overlapping URL logic.

## Rationale

Reusing the existing import flow avoids coordinated SimpleDMS changes and keeps
the user experience consistent. A shared package would add build and release
coupling to the different platform stacks for little reusable code.

## Consequences

SimpleDMS needs source-specific validation and authentication but no new route.
The integration imports one file at a time; batching, synchronization, metadata
mapping, and completion polling are outside this implementation.
