import summary from "@/lib/portfolio-recovery.generated.json";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { Timestamp } from "@/ui/display/Timestamp";

export function CandidateRecoveryEvidence() {
  return <section aria-labelledby="candidate-recovery-heading">
    <h3 id="candidate-recovery-heading">Recorded candidate restore</h3>
    <p>This separate local drill used synthetic records. It does not prove that the normal database has a current backup or that a managed recovery target is met.</p>
    <dl className="evidence-list">
      <div><dt>Recorded result</dt><dd>Restore, financial checks, access controls, migration retry, and cleanup passed.</dd></div>
      <div><dt>Completed</dt><dd><Timestamp value={summary.completed_at} /></dd></div>
      <div><dt>Run</dt><dd><CopyControl value={summary.run_id} label="Copy candidate recovery run ID" /></dd></div>
      <div><dt>Source snapshot</dt><dd><CopyControl value={summary.source_manifest_sha256} label="Copy candidate recovery source hash" /></dd></div>
      <div><dt>Schema history</dt><dd>{summary.migration_count} applied migrations.</dd></div>
      <div><dt>Local elapsed time</dt><dd>{summary.backup_and_validation_seconds.toFixed(2)} seconds from backup start through the full named validation profile. Database restore setup alone took {summary.restore_setup_seconds.toFixed(2)} seconds.</dd></div>
    </dl>
    <p>Run <code>node scripts/portfolio/run-recovery.mjs</code> on the host to create a new isolated proof. This page cannot start a restore or read a backup file.</p>
  </section>;
}
