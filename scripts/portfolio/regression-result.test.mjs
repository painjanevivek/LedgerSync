import test from 'node:test';
import assert from 'node:assert/strict';
import { regressionExcluded, regressionResult } from './regression-result.mjs';

const output = (...records) => records.map(record => JSON.stringify(record)).join('\n');
test('complete discovered inventory must pass exactly once and no skip can hide work', () => {
  const valid = [{ Action: 'pass', Test: 'TestA' }, { Action: 'pass', Test: 'TestA/sub' }, { Action: 'pass', Test: 'TestB' }];
  assert.equal(regressionResult(output(...valid), ['TestA', 'TestB'], 0).status, 'passed');
  for (const records of [valid.slice(0, 2), [...valid, valid[0]], [...valid, { Action: 'skip', Test: 'TestA/sub' }], [...valid, { Action: 'fail' }]]) assert.equal(regressionResult(output(...records), ['TestA', 'TestB'], 0).status, 'failed');
  assert.equal(regressionResult(output(...valid), ['TestA', 'TestB'], 1).status, 'failed');
  assert.throws(() => regressionResult('', [], 0));
  const excluded = new RegExp(regressionExcluded);
  for (const name of ['TestOutboxCrashHelper', 'TestPortfolioCapacity', 'TestPortfolioRecoverySeed', 'TestPortfolioRecoveryRestored', 'TestPortfolioRecoveryMigrations', 'TestPortfolioDeveloperIntegration', 'TestPortfolioOptionalFeatures']) assert.equal(excluded.test(name), true);
  for (const name of ['TestGeneratedFinancialModel', 'TestOutboxProcessCrashRecovery', 'TestDatabaseRoleCapabilities']) assert.equal(excluded.test(name), false);
});
