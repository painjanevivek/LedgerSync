import { existsSync, writeFileSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import { loadExampleConfig } from './config.ts';
import { ReceiverStore, receiverServer } from './receiver.ts';
import { privateTraceSink } from './diagnostic.ts';

try {
  if (process.argv.length !== 3) throw new Error('Protected configuration path required');
  const config = loadExampleConfig(process.argv[2]), receiver = config.receiver;
  if (!receiver || !isAbsolute(receiver.statePath) || receiver.readyPath && !isAbsolute(receiver.readyPath) || receiver.failurePath && !isAbsolute(receiver.failurePath)) throw new Error('Explicit protected receiver paths required');
  const store = new ReceiverStore(receiver.statePath, { [receiver.keyId]: Buffer.from(receiver.keyBase64, 'base64') });
  const traceSink = receiver.tracePath ? privateTraceSink(receiver.tracePath, receiver.statePath) : undefined;
  const server = receiverServer(store, () => Boolean(receiver.failurePath && existsSync(receiver.failurePath)), traceSink);
  server.requestTimeout = 5000; server.headersTimeout = 5000; server.maxHeadersCount = 32;
  server.listen(0, '127.0.0.1', () => {
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('Loopback receiver allocation failed');
    const ready = { url: `http://127.0.0.1:${address.port}/hooks`, pid: process.pid, scope: 'local synthetic receiver only' };
    if (receiver.readyPath) writeFileSync(receiver.readyPath, JSON.stringify(ready), { flag: 'wx', mode: 0o600 });
    console.log(JSON.stringify(ready));
  });
  const stop = () => { server.closeAllConnections(); server.close(() => { store.close(); process.exitCode = 0; }); };
  process.once('SIGINT', stop); process.once('SIGTERM', stop);
} catch { console.error('Local receiver did not start. Check protected receiver configuration.'); process.exitCode = 1; }
