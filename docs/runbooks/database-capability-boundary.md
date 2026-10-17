# Database capability boundary

This is the current replacement for the earlier branch runbook. The old migration numbers are historical. Do not add their files again or apply them to the normal database.

Use the [workload capability matrix](../security/database-capability-matrix.md), [tenant cutover procedure](../security/tenant-context-cutover.md) and [migration compatibility procedure](migration-compatibility.md). The current source includes controlled financial commands, fixed function owners, explicit execution grants and forced row policies. Current opening-import commands are in migration `000064`; the earlier branch called this work `000037`.

1. Keep Docker and deployments stopped during this consolidation. A Git merge is not approval to migrate a database.
2. Before a later release, preserve and verify the source, images, migration history and backup. Use a disposable restore target.
3. Review the exact migration files and `deploy/postgres/roles.sql`. Never use an old branch's grant script against newer financial code.
4. Run fresh-install, populated-upgrade and actual workload-role tests. Verify both allowed commands and denied direct writes. A health response does not prove the capability boundary.
5. Stop on an unexpected role flag, inherited grant, public execution permission, missing audit event or financial mismatch. Preserve evidence and repair forward.
6. Keep break-glass access separate from normal workload authority. Follow the matrix and incident procedure; do not add standing emergency grants to source.

Record the exact tested source and database identity. Old CI and phase records retain their original scope. Normal schema64 data and all original volumes remain preserved during this work.
