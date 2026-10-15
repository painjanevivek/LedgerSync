# ADR 0008: Simple-first presentation and shared production limits

## Status

Accepted.

## Context

Ledger evidence must stay exact and auditable, but exposing every system term and identifier in the primary interface increases operator stress and unsafe retry risk. Process-local BFF rate limits also do not provide a global protection boundary when the frontend runs on multiple serverless instances.

## Decision

- Simple view is the safe default. Expert view is a tenant-and-operator-scoped preference backed by PostgreSQL.
- Both modes use the same domain models, authorization, commands, and presentation adapters. A mode may change disclosure and density only; it cannot change financial calculations or capabilities.
- Urgent errors, mismatches, and unknown financial outcomes remain visible in every mode.
- Presentation follows outcome, explanation, then evidence. Raw identifiers and diagnostics are available through explicit technical details.
- CSS uses ordered cascade layers: tokens, reset, foundations, primitives, patterns, features, utilities, and overrides.
- Local development uses the in-memory fixed-window limiter. Production-capable and Vercel runtimes use an atomic Upstash Redis counter with HMAC-derived keys.
- Missing or unavailable shared limiting fails closed with `503`; an exhausted valid window returns `429` with `Retry-After`.
- The PostgreSQL-backed Go limiter remains the authoritative financial API protection.

## Consequences

The common workflows are calmer without weakening evidence or authorization. Production requires a shared limiter and an environment-specific namespace before traffic is accepted. Local work requires neither Upstash nor any cloud resource, and this decision creates or changes no secrets.
