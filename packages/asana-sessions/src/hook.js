// The provider plugin hook: `asana-sessions hook <provider>` reads one native
// UserPromptSubmit or Stop event and links the conversation to the single
// Asana task URL in the submitted prompt. It never blocks the prompt: every
// outcome exits 0, and stdout carries only an optional nonblocking
// {"systemMessage": ...} notice that both providers show to the user.
import { isAbsolute } from 'node:path';
import { display } from './display.js';
import { ProviderError, UsageError } from './errors.js';
import { link } from './engine.js';
import { readBounded } from './input.js';
import { taskGIDsInText } from './marker.js';
import { savePending, sweepPending, takePending } from './pending.js';

export const MAX_EVENT = 1024 * 1024;
// DEADLINE_MS stays under the plugins' 15-second native hook timeout.
export const DEADLINE_MS = 10000;

const EVENTS = new Set(['UserPromptSubmit', 'Stop']);

export async function hook(provider, c, { deadlineMs = DEADLINE_MS } = {}) {
  const notice = (text) => c.stdout.write(`${JSON.stringify({ systemMessage: `asana-sessions: ${display(text).slice(0, 500)}` })}\n`);
  c.abort ??= new AbortController();
  let timer;
  const late = new Promise((resolve) => {
    timer = setTimeout(() => {
      c.abort?.abort();
      c.stdin.destroy();
      resolve('linking timed out; link status unknown; check the conversation title');
    }, deadlineMs);
  });
  try {
    const message = await Promise.race([handle(provider, c), late]);
    if (message) {
      notice(message);
    }
  } catch (err) {
    notice(`linking failed: ${err instanceof ProviderError || err instanceof UsageError ? err.message : 'unexpected error'}`);
  } finally {
    clearTimeout(timer);
  }
  return 0;
}

// handle returns a notice, or '' when there is nothing to report.
async function handle(provider, c) {
  let p = Object.hasOwn(c.providers, provider) ? c.providers[provider] : null;
  if (!p) {
    throw new UsageError(`unsupported provider ${JSON.stringify(display(provider))}`);
  }
  const event = parseEvent(await readBounded(c.stdin, MAX_EVENT, () => new UsageError('hook event exceeds 1 MiB; skipped')), p);
  // Hook commands may run in a login shell whose profile sets another home;
  // the event's transcript names the one the provider process uses.
  const home = p.homeEnv?.(event.transcript, event.sessionId);
  if (!home && p.homeEnv) {
    throw new UsageError('hook transcript does not identify the provider home and session; skipped');
  }
  if (home && c.makeProviders) {
    p = c.makeProviders({ ...c.env, ...home })[provider];
  }
  const lockOptions = { env: c.env, signal: c.abort.signal };
  if (event.name === 'Stop') {
    if (provider !== 'claude') {
      throw new UsageError('unsupported hook event for codex; skipped');
    }
    const gid = await takePending(c.env, provider, event.sessionId);
    return gid ? linkNow(p, event.sessionId, gid, lockOptions, true) : '';
  }
  const gids = taskGIDsInText(event.prompt);
  if (gids.length === 0) {
    return '';
  }
  if (gids.length > 1) {
    return `the prompt links ${gids.length} Asana tasks (${gids.join(', ')}); not linking automatically. Ask to link one task explicitly.`;
  }
  await sweepPending(c.env);
  // A new conversation's first prompt arrives before the provider saves it;
  // finish after the reply, when the Stop hook runs.
  const saved = await p.read(event.sessionId).catch((err) => {
    if (provider === 'claude' && err instanceof ProviderError && err.code === 'unavailable'
      && err.message === 'Claude Code is not installed or has no history') {
      return null;
    }
    throw err;
  });
  c.abort.signal.throwIfAborted();
  if (!saved && provider === 'claude') {
    const other = await savePending(c.env, provider, event.sessionId, gids[0]);
    return other ? `task ${other} is already waiting to link to this conversation; not linking ${gids[0]}` : '';
  }
  return linkNow(p, event.sessionId, gids[0], lockOptions, false);
}

function parseEvent(text, p) {
  let e;
  try {
    e = JSON.parse(text);
  } catch {
    throw new UsageError('hook event is not JSON; skipped');
  }
  if (e === null || typeof e !== 'object' || !EVENTS.has(e.hook_event_name)) {
    throw new UsageError('unsupported hook event; skipped');
  }
  if (typeof e.cwd !== 'string' || !isAbsolute(e.cwd)) {
    throw new UsageError('hook event has no absolute cwd; skipped');
  }
  if (typeof e.session_id !== 'string' || !p.validID(e.session_id)) {
    throw new UsageError(`hook event has no valid ${p.name} session ID; skipped`);
  }
  if (e.hook_event_name === 'UserPromptSubmit' && typeof e.prompt !== 'string') {
    throw new UsageError('hook event has no prompt; skipped');
  }
  return { name: e.hook_event_name, sessionId: e.session_id, prompt: e.prompt, transcript: e.transcript_path };
}

// linkNow never relinks: a different existing link is reported, not replaced.
async function linkNow(p, sessionId, gid, lockOptions, deferred) {
  try {
    const r = await link({ [p.name]: p }, { provider: p.name, sessionId, gid }, lockOptions);
    return r.changed ? `linked this conversation to Asana task ${gid}` : '';
  } catch (err) {
    if (err instanceof ProviderError && err.code === 'metadata_conflict') {
      return `${err.message.replace(/; pass --replace.*$/, '')}; not linking ${gid}. Ask to change the linked task explicitly.`;
    }
    if (deferred && /was not found/.test(err?.message ?? '')) {
      return `this conversation was not found in ${p.name} history; not linked to task ${gid}`;
    }
    throw err;
  }
}
