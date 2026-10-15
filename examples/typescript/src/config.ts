import { readFileSync, lstatSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import type { ClientOptions } from '../../../contracts/generated/typescript/ledgersync.ts';

export type ExampleConfig = { statePath: string; baseUrl: string; principals: Record<string, { token: string }>; receiver?: { statePath: string; keyId: string; keyBase64: string; readyPath?: string; failurePath?: string } };
export function readPrivateJSON(path: string, maximum = 16_384): unknown {
  const info = lstatSync(path); if (!info.isFile() || info.size > maximum) throw new Error('Bounded regular private file required');
  return JSON.parse(readFileSync(path, 'utf8'));
}
export function loadExampleConfig(path: string): ExampleConfig {
  const config = readPrivateJSON(path) as ExampleConfig;
  if (!config || !isAbsolute(config.statePath) || typeof config.baseUrl !== 'string' || !config.principals || Object.keys(config.principals).length < 1 || Object.keys(config.principals).length > 8 || Object.entries(config.principals).some(([alias, value]) => !/^[a-z][a-z0-9-]{0,31}$/.test(alias) || typeof value.token !== 'string' || value.token.length < 32 || value.token.length > 16_384)) throw new Error('Protected own-principal client bindings required');
  return config;
}
export function principalBindings(config: ExampleConfig): Record<string, ClientOptions> {
  return Object.fromEntries(Object.entries(config.principals).map(([alias, value]) => [alias, { baseUrl: config.baseUrl, token: value.token, timeoutMilliseconds: 5000 }]));
}
