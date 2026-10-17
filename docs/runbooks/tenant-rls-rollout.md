# Tenant row-policy rollout

The earlier branch used migrations `000039` and `000040` for an expand-and-force sequence. Current source uses the later scoped-context, controlled-owner and forced-policy design. Do not restore the old missing-context compatibility path or duplicate the earlier migration numbers.

Use [context adoption](../security/tenant-context-adoption.md), [the cutover procedure](../security/tenant-context-cutover.md) and [the capability matrix](../security/database-capability-matrix.md).

1. Keep runtime execution stopped for this consolidation. Do not start the normal stack to test a Git update.
2. In a later approved disposable target, check the complete current migration sequence and role script together. The context cutover starts at `000044`–`000046`; session and preference policies also use `000062` and `000063`. Extension policies have their own migrations and tests.
3. Verify missing context, another tenant, another actor, rollback, retry and connection-pool reuse. Include actual restricted workload identities. Do not substitute administrator tests.
4. Verify queue owners, controlled function owners and helper permissions. Confirm that scoped workers cannot gain general financial write authority.
5. Use the current inventory tests to count covered tables and functions. Do not copy an obsolete table count from an older branch.
6. Stop on cross-tenant visibility, retained pool context, missing forced policy or unauthorized mutation. Keep the failed evidence and repair forward. Do not disable row security as rollback.

The shared application workload remains trusted to install verified identity context. Database settings are not independent credentials against a compromised workload. No managed identity or normal-database cutover is approved by this document.
