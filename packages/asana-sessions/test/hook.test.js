import assert from 'node:assert/strict';
import { readdir, readFile, rm, utimes } from 'node:fs/promises';
import { join } from 'node:path';
import { PassThrough, Readable } from 'node:stream';
import test from 'node:test';
import { hook, MAX_EVENT } from '../src/hook.js';
import { taskGIDsInText } from '../src/marker.js';
import { savePending, takePending, sweepPending, RETENTION_MS } from '../src/pending.js';
import { ID, fakeProvider, lockEnv, session } from './fakes.js';

const event = (extra = {}) => ({ hook_event_name: 'UserPromptSubmit', session_id: ID, cwd: '/tmp', prompt: 'https://app.asana.com/0/1/42', ...extra });
async function run(p, input, env, options) {
  let output = '';
  const c = { providers: { [p.name]: p }, env, stdin: typeof input === 'string' ? Readable.from([input]) : input,
    stdout: { write: (s) => { output += s; } }, abort: new AbortController() };
  assert.equal(await hook(p.name, c, options), 0);
  return output ? JSON.parse(output).systemMessage : '';
}

test('URL extraction uses only supported task links and unique GIDs', () => {
  assert.deepEqual(taskGIDsInText('42 [task](https://app.asana.com/0/1/42). <https://app.asana.com/1/1/task/42> https://app.asana.com/0/1/43!'), ['42', '43']);
  assert.deepEqual(taskGIDsInText('https://evil.test/0/1/42 https://app.asana.com.evil/0/1/42 https://app.asana.com/0/1/list'), []);
});

test('prompt hook verifies exact session, preserves title and tag, never replaces', async (t) => {
  const { env } = await lockEnv(); t.after(() => rm(env.XDG_STATE_HOME, { recursive: true, force: true }));
  const p = fakeProvider('codex', [session({ name: 'Work', tag: 'keep' })]);
  const send = (e) => run(p, JSON.stringify(event(e)), env);
  assert.match(await send(), /linked this conversation/);
  assert.equal(p.store.get(ID).name, 'Work [asana:42]');
  assert.equal(p.store.get(ID).tag, 'keep');
  assert.equal(await send(), '');
  assert.match(await send({ prompt: 'https://app.asana.com/0/1/43' }), /already links ticket 42/);
  assert.equal(p.store.get(ID).name, 'Work [asana:42]');
  assert.match(await send({ prompt: 'https://app.asana.com/0/1/42 https://app.asana.com/0/1/43' }), /2 Asana tasks/);
  assert.equal(await send({ prompt: 'no task', system: 'https://app.asana.com/0/1/43' }), '');
});

test('Claude defers missing first record; Stop consumes only pending GID', async (t) => {
  const { env } = await lockEnv(); t.after(() => rm(env.XDG_STATE_HOME, { recursive: true, force: true }));
  const p = fakeProvider('claude');
  assert.equal(await run(p, JSON.stringify(event()), env), '');
  const files = await readdir(join(env.XDG_STATE_HOME, 'asana-sessions/pending'));
  const pending = JSON.parse(await readFile(join(env.XDG_STATE_HOME, 'asana-sessions/pending', files[0])));
  assert.deepEqual(Object.keys(pending).sort(), ['created_at', 'provider', 'session_id', 'task_gid']);
  p.store.set(ID, session({ provider: 'claude', name: 'Generated title' }));
  assert.match(await run(p, JSON.stringify(event({ hook_event_name: 'Stop', prompt: 'https://app.asana.com/0/1/43' })), env), /task 42/);
  assert.equal(await takePending(env, 'claude', ID), '');
  assert.equal(p.store.get(ID).name, 'Generated title [asana:42]');
});

test('malformed, oversize, unreadable and unknown events never block or rename', async (t) => {
  const { env } = await lockEnv(); t.after(() => rm(env.XDG_STATE_HOME, { recursive: true, force: true }));
  const p = fakeProvider('codex');
  for (const text of ['no JSON', '[]', JSON.stringify(event({ session_id: '../bad' })), JSON.stringify(event({ cwd: '' })), JSON.stringify(event({ prompt: null })), 'x'.repeat(MAX_EVENT + 1)]) {
    assert.match(await run(p, text, env), /skipped/);
  }
  assert.match(await run(p, JSON.stringify(event()), env), /not found/);
  assert.equal(p.calls.some(([method]) => method === 'rename'), false);
  p.read = async () => { throw new Error('private data'); };
  assert.match(await run(p, JSON.stringify(event()), env), /unexpected error/);
});

test('deadline aborts provider operations and a stalled stdin', async () => {
  const p = fakeProvider('codex');
  const stream = new PassThrough();
  assert.match(await run(p, stream, {}, { deadlineMs: 10 }), /timed out/);
  assert.equal(stream.destroyed, true);
});

test('pending conflicts serialize; expired entries are discarded and swept', async (t) => {
  const { env } = await lockEnv(); t.after(() => rm(env.XDG_STATE_HOME, { recursive: true, force: true }));
  const results = await Promise.all(['42', '43'].map((gid) => savePending(env, 'claude', ID, gid)));
  assert.equal(results.filter(Boolean).length, 1);
  assert.equal(await takePending(env, 'claude', ID), results.find(Boolean));
  await savePending(env, 'claude', ID, '42');
  const dir = join(env.XDG_STATE_HOME, 'asana-sessions/pending');
  for (const f of await readdir(dir)) await utimes(join(dir, f), 0, 0);
  await sweepPending(env, { now: Date.now() + RETENTION_MS });
  assert.deepEqual(await readdir(dir), []);
});

test('timeout during metadata read cannot cause a later rename', async (t) => {
  const { env } = await lockEnv(); t.after(() => rm(env.XDG_STATE_HOME, { recursive: true, force: true }));
  const p = fakeProvider('codex', [session({ name: 'Keep' })]);
  let finish;
  p.read = () => new Promise((resolve) => { finish = resolve; });
  assert.match(await run(p, JSON.stringify(event()), env, { deadlineMs: 10 }), /timed out/);
  finish(session({ name: 'Keep' }));
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(p.calls.some(([method]) => method === 'rename'), false);
});

test('event transcript must match the exact session before metadata is touched', async () => {
  const { createClaude } = await import('../src/claude.js');
  const { createCodex } = await import('../src/codex.js');
  for (const p of [createClaude(), createCodex()]) {
    p.read = () => { throw new Error('must not read'); };
    assert.match(await run(p, JSON.stringify(event({ transcript_path: `/wrong/${ID}.jsonl` })), {}), /does not identify/);
  }
});
