import { DatabaseSync } from 'node:sqlite';
import { dirname } from 'node:path';
import { mkdirSync, existsSync, lstatSync, chmodSync } from 'node:fs';

// Use a protected local directory. No database, journal or receipt belongs in Git.
export function openState(path: string): DatabaseSync {
  mkdirSync(dirname(path), { recursive: true, mode: 0o700 });
  if (lstatSync(dirname(path)).isSymbolicLink() || existsSync(path) && !lstatSync(path).isFile()) throw new Error('Regular private state path required');
  const database = new DatabaseSync(path);
  chmodSync(path, 0o600);
  database.exec('PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;');
  return database;
}
