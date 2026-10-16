// Serializes this package's own metadata writes per provider/session. It does
// not protect against a provider's concurrent title writer.
import { createHash } from 'node:crypto';
import { mkdir, rm, stat } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';
import { ProviderError } from './errors.js';

export function stateDir(env = process.env) {
  return join(env.XDG_STATE_HOME || join(homedir(), '.local', 'state'), 'asana-sessions');
}

export async function withLock(provider, sessionId, fn, { env = process.env, timeoutMs = 3000, staleMs = 30000 } = {}) {
  const dir = join(stateDir(env), 'locks');
  await mkdir(dir, { recursive: true, mode: 0o700 });
  const path = join(dir, `${provider}-${createHash('sha256').update(sessionId).digest('hex').slice(0, 32)}.lock`);
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    try {
      await mkdir(path, { mode: 0o700 });
      break;
    } catch (err) {
      if (err.code !== 'EEXIST') {
        throw err;
      }
    }
    const held = await stat(path).catch(() => null);
    if (held && Date.now() - held.mtimeMs > staleMs) {
      await rm(path, { recursive: true, force: true });
    } else if (Date.now() > deadline) {
      throw new ProviderError('provider_error', 'another asana-sessions write to this conversation is in progress');
    } else {
      await sleep(50);
    }
  }
  try {
    return await fn();
  } finally {
    await rm(path, { recursive: true, force: true });
  }
}
