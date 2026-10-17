export const humanAcceptanceTasks = Object.freeze(['purpose', 'boundaries', 'first-journey', 'transfer-review', 'unknown-result', 'investigation', 'reproduce-failure', 'safe-handover']);

// This validates a submitted record. It cannot authenticate a human or certify
// their independence; the maintainer must inspect the actual record origin.
export function checkHumanAcceptance(record) {
  if (record?.schema !== 'ledgersync-human-acceptance/v1' || record.purpose !== 'human_observations' || !/^[a-f0-9]{64}$/.test(record.source_manifest_sha256 ?? '') || !Number.isFinite(Date.parse(record.reviewed_at)) || !/^reviewer-[a-z0-9-]{3,48}$/.test(record.reviewer?.reference ?? '') || record.reviewer.unfamiliar_with_project !== true || record.reviewer.is_author !== false || record.reviewer.human_attestation !== true || !Array.isArray(record.tasks) || record.tasks.length !== humanAcceptanceTasks.length || typeof record.reviewer_explanation !== 'string' || record.reviewer_explanation.trim().length < 40 || record.reviewer_explanation.length > 2000) throw Error('Actual bounded independent reviewer observations required; templates and author records cannot pass');
  const seen = new Set();
  for (const task of record.tasks) {
    if (!humanAcceptanceTasks.includes(task.id) || seen.has(task.id) || !['passed', 'failed', 'not_run'].includes(task.result) || typeof task.help_required !== 'boolean' || !Number.isSafeInteger(task.elapsed_seconds) || task.elapsed_seconds < 0 || task.elapsed_seconds > 7200 || typeof task.observation !== 'string' || task.observation.trim().length < 10 || task.observation.length > 1000) throw Error('Complete bounded task observations required');
    seen.add(task.id);
  }
  return { status: 'record_valid', tasks_complete: record.tasks.every(task => task.result === 'passed' && !task.help_required), human_identity: 'requires_maintainer_origin_review', acceptance: 'not_automatically_granted' };
}
