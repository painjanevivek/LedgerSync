# Backup and PITR operating policy

The policy in [postgres-pitr-policy.yaml](../../deploy/backup/postgres-pitr-policy.yaml) is a release requirement, not an enabled backup system by itself.

Before shared production, the platform owner must attach managed-provider evidence that confirms encrypted continuous WAL archiving, 35-day point-in-time retention, a separate backup trust boundary, and backup age below 15 minutes. Run an isolated restore at least every 30 days. A missing or failed drill blocks release.
