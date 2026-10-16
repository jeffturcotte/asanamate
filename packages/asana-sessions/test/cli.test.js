import assert from 'node:assert/strict';
import { mkdtemp, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Readable } from 'node:stream';
import test from 'node:test';
import { VERSION, main } from '../src/cli.js';
import { ID, failing, fakeIO, fakeProvider, lockEnv, session } from './fakes.js';

function sink() {
  const out = { text: '', write: (s) => { out.text += s; return true; } };
  return out;
}

async function run(argv, { providers, stdin = '', env = {}, io = fakeIO([], { interactive: false }) } = {}) {
  const stdout = sink();
  const stderr = sink();
  const code = await main(argv, { stdout, stderr, stdin: Readable.from([stdin]), env, providers, io });
  return { code, stdout: stdout.text, stderr: stderr.text };
}

const linked = () => ({
  claude: failing('claude', 'unavailable'),
  codex: fakeProvider('codex', [session({ name: 'Fix \u001b[2Jsorting [asana:42]', title: 'Fix \u001b[2Jsorting [asana:42]', cwd: tmpdir() })]),
});

test('help and version need no providers', async () => {
  assert.equal((await run(['help'])).code, 0);
  assert.match((await run([])).stdout, /asana-sessions list/);
  assert.equal((await run(['version'])).stdout, `${VERSION}\n`);
});

test('usage errors exit 2', async () => {
  for (const argv of [['bogus'], ['list'], ['list', 'not-a-task'], ['link', '--provider', 'codex'], ['list', '42', '--nope'], ['asanamate']]) {
    const r = await run(argv, { providers: linked() });
    assert.equal(r.code, 2, argv.join(' '));
    assert.match(r.stderr, /asana-sessions help/);
  }
});

test('list prints sanitized human rows and diagnostics', async () => {
  const r = await run(['list', 'https://app.asana.com/0/1/42'], { providers: linked() });
  assert.equal(r.code, 0);
  assert.match(r.stdout, new RegExp(`codex  ${ID}`));
  assert.match(r.stdout, /Fix sorting \[asana:42\]/);
  assert.doesNotMatch(r.stdout, /\u001b/);
  assert.match(r.stdout, /info: claude: unavailable/);
});

test('list --json returns the versioned record shape and partial exit status', async () => {
  const ok = await run(['list', '42', '--json'], { providers: linked() });
  assert.equal(ok.code, 0);
  const body = JSON.parse(ok.stdout);
  assert.equal(body.version, 1);
  assert.equal(body.conversations[0].session_id, ID);
  assert.equal(body.status, undefined);

  const partial = await run(['list', '42', '--json'], { providers: { claude: failing('claude', 'provider_error'), codex: linked().codex } });
  assert.equal(partial.code, 1);
  assert.equal(JSON.parse(partial.stdout).conversations.length, 1);

  const none = await run(['list', '42', '--json'], { providers: { claude: failing('claude', 'unavailable'), codex: failing('codex', 'unavailable') } });
  assert.equal(none.code, 1);
  assert.deepEqual(JSON.parse(none.stdout).conversations, []);
});

test('link and unlink print structured results', async () => {
  const providers = { codex: fakeProvider('codex', [session({ name: 'Fix' })]) };
  const { env } = await lockEnv();
  const linkedRun = await run(['link', '--provider', 'codex', '--session', ID, '--task', '42'], { providers, env });
  assert.equal(linkedRun.code, 0);
  assert.deepEqual(JSON.parse(linkedRun.stdout), { provider: 'codex', session_id: ID, task_gid: '42', title: 'Fix [asana:42]', changed: true });
  const conflict = await run(['link', '--provider', 'codex', '--session', ID, '--task', '7'], { providers, env });
  assert.equal(conflict.code, 1);
  assert.match(conflict.stderr, /--replace/);
  const unlinked = await run(['unlink', '--provider', 'codex', '--session', ID], { providers, env });
  assert.equal(JSON.parse(unlinked.stdout).title, 'Fix');
});

test('asanamate list adapts the protocol request', async () => {
  const request = JSON.stringify({ version: 1, operation: 'list', asana: { task_gid: '42' }, extra: true });
  const ok = await run(['asanamate', 'list'], { providers: linked(), stdin: request });
  assert.equal(ok.code, 0);
  assert.equal(JSON.parse(ok.stdout).conversations.length, 1);

  // A partial lookup is still a valid protocol response.
  const partial = await run(['asanamate', 'list'], { providers: { claude: failing('claude', 'permission_denied'), codex: linked().codex }, stdin: request });
  assert.equal(partial.code, 0);
  assert.equal(JSON.parse(partial.stdout).diagnostics[0].code, 'permission_denied');

  const none = await run(['asanamate', 'list'], { providers: { claude: failing('claude', 'unavailable'), codex: failing('codex', 'unavailable') }, stdin: request });
  assert.equal(none.code, 1);
  assert.equal(none.stdout, '');
  assert.match(none.stderr, /doctor/);

  for (const bad of ['nope', '[]', '{"version":2,"operation":"list","asana":{"task_gid":"42"}}', '{"version":1,"operation":"open","asana":{"task_gid":"42"}}', '{"version":1,"operation":"list","asana":{"task_gid":"4a"}}', 'x'.repeat(70000)]) {
    assert.equal((await run(['asanamate', 'list'], { providers: linked(), stdin: bad })).code, 2, bad.slice(0, 40));
  }
});

test('asanamate open reads the private snapshot and keeps stdin for the terminal', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'asana-sessions-open-'));
  const path = join(dir, 'conversation.json');
  const conversation = { provider: 'codex', session_id: ID, task_gid: '42', title: 'x', activity: 'unknown', archived: false };
  await writeFile(path, JSON.stringify({ version: 1, operation: 'open', asana: { task_gid: '42' }, conversation }));
  const env = { ASANAMATE_CONVERSATION_JSON: path, ASANAMATE_CONVERSATION_PROVIDER: 'codex', ASANAMATE_CONVERSATION_ID: ID };

  const io = fakeIO(['y']);
  const r = await run(['asanamate', 'open'], { providers: linked(), env, io });
  assert.equal(r.code, 3);
  assert.deepEqual(io.spawned.map((s) => s.args), [['resume', ID]]);

  const mismatch = await run(['asanamate', 'open'], { providers: linked(), env: { ...env, ASANAMATE_CONVERSATION_ID: 'other' }, io: fakeIO(['y']) });
  assert.equal(mismatch.code, 2);
  assert.equal((await run(['asanamate', 'open'], { providers: linked(), env: {} })).code, 2);

  await writeFile(path, JSON.stringify({ version: 1, operation: 'open', asana: { task_gid: '42' }, conversation: { ...conversation, task_gid: '7' } }));
  assert.equal((await run(['asanamate', 'open'], { providers: linked(), env })).code, 2);
});
