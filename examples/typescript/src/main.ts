import { loadExampleConfig, principalBindings, readPrivateJSON } from './config.ts';
import { IntentStore, type Intent } from './intents.ts';
import { PreparedClient } from './client.ts';

let store: IntentStore | undefined;
try {
  const [action, configPath, input] = process.argv.slice(2);
  if (!['prepare', 'send', 'inspect'].includes(action) || !configPath || !input || process.argv.length !== 5) throw new Error('Usage: node src/main.ts <prepare|send|inspect> <private-config.json> <intent-file|original-key>');
  const config = loadExampleConfig(configPath); store = new IntentStore(config.statePath);
  const client = new PreparedClient(store, principalBindings(config));
  if (action === 'prepare') { const intent = readPrivateJSON(input) as Intent; client.prepare(intent); console.log(JSON.stringify({ state: 'prepared', key: intent.key, next: 'Inspect the saved intent, then send this original key.' })); }
  else if (action === 'inspect') console.log(JSON.stringify(store.get(input), null, 2));
  else console.log(JSON.stringify(await client.send(input), null, 2));
} catch { console.error('Example stopped. Check command arguments, protected configuration, and original saved intent. No automatic retry or new key was made.'); process.exitCode = 1; }
finally { store?.close(); }
