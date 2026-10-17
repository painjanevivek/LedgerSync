export const regressionExcluded = '^Test(?:Portfolio(?:Capacity|Recovery.*|ExtensionRecovery.*|DeveloperIntegration|OptionalFeatures)|OutboxCrashHelper)$';

export function regressionResult(lines, expected, exitCode) {
  if (typeof lines !== 'string' || Buffer.byteLength(lines) > 8 * 1024 * 1024 || !Array.isArray(expected) || !expected.length || new Set(expected).size !== expected.length || expected.some(name => !/^Test[A-Za-z0-9_]+$/.test(name))) throw new Error('Bounded complete test inventory required');
  const records = lines.trim().split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
  const passed = records.filter(record => record.Action === 'pass' && expected.includes(record.Test));
  const valid = exitCode === 0 && !records.some(record => ['fail', 'skip'].includes(record.Action)) && expected.every(name => passed.filter(record => record.Test === name).length === 1);
  return { status: valid ? 'passed' : 'failed', ...(valid ? {} : { reason: 'Complete discovered non-opt-in inventory did not pass without failure or skip.' }), checks: [{ id: 'complete-runtime-inventory', status: valid ? 'passed' : 'failed', ...(valid ? {} : { reason: 'Inspect the preserved test inventory and JSON output.' }) }] };
}
