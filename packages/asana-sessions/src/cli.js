// Public command dispatch. Exit codes: 0 success or complete lookup, 1 failure
// or partial/unavailable lookup, 2 usage error.
import { execFile } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { createInterface } from 'node:readline/promises';
import { fileURLToPath } from 'node:url';
import { parseArgs, promisify } from 'node:util';
import { asanamateList, asanamateOpen } from './asanamate.js';
import { display } from './display.js';
import { Cancelled, createProviders, link, lookup, open, unlink } from './engine.js';
import { ProviderError, UsageError } from './errors.js';
import { parseTask } from './marker.js';

const pkg = JSON.parse(await readFile(join(dirname(fileURLToPath(import.meta.url)), '..', 'package.json'), 'utf8'));

export const VERSION = pkg.version;

const USAGE = `asana-sessions ${VERSION}: find and resume Claude Code and Codex conversations by Asana ticket

Usage:
  asana-sessions list <task-url-or-gid> [--json]
  asana-sessions open <task-url-or-gid> --provider <claude|codex> --session <id>
  asana-sessions link --provider <claude|codex> --session <id> --task <task-url-or-gid> [--replace]
  asana-sessions unlink --provider <claude|codex> --session <id>
  asana-sessions doctor
  asana-sessions help | version

  asana-sessions asanamate list   asanamate conversations.command adapter
  asana-sessions asanamate open   asanamate conversations.open adapter
`;

// terminalIO asks on the controlling terminal. Prompts and notices go to stderr.
export function terminalIO(stdin = process.stdin, stderr = process.stderr) {
  return {
    interactive: Boolean(stdin.isTTY && stderr.isTTY),
    say: (line) => stderr.write(`${line}\n`),
    async ask(question) {
      const rl = createInterface({ input: stdin, output: stderr });
      try {
        return await rl.question(question);
      } finally {
        rl.close();
      }
    },
  };
}

export async function main(argv, ctx = {}) {
  const c = {
    stdout: process.stdout, stderr: process.stderr, stdin: process.stdin, env: process.env,
    ...ctx,
  };
  c.providers ??= createProviders({ env: c.env, version: VERSION });
  c.io ??= terminalIO(c.stdin, c.stderr);
  try {
    return await dispatch(argv, c);
  } catch (err) {
    if (err instanceof Cancelled) {
      c.stderr.write('asana-sessions: cancelled\n');
      return 1;
    }
    c.stderr.write(`asana-sessions: ${display(err.message)}\n`);
    if (err instanceof UsageError) {
      c.stderr.write('Run `asana-sessions help` for usage.\n');
      return 2;
    }
    return 1;
  }
}

const OPTIONS = {
  json: { type: 'boolean' },
  provider: { type: 'string' },
  session: { type: 'string' },
  task: { type: 'string' },
  replace: { type: 'boolean' },
};

function parse(args, allowed, positionals = 0) {
  let parsed;
  try {
    parsed = parseArgs({ args, options: Object.fromEntries(allowed.map((k) => [k, OPTIONS[k]])), allowPositionals: true, strict: true });
  } catch (err) {
    throw new UsageError(err.message);
  }
  if (parsed.positionals.length !== positionals) {
    throw new UsageError(`expected ${positionals} argument(s)`);
  }
  for (const name of allowed) {
    if (OPTIONS[name].type === 'string' && parsed.values[name] === undefined) {
      throw new UsageError(`--${name} is required`);
    }
  }
  return { ...parsed.values, args: parsed.positionals };
}

async function dispatch([command, ...rest], c) {
  const json = (value) => c.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
  switch (command) {
    case undefined:
    case 'help':
    case '--help':
    case '-h':
      c.stdout.write(USAGE);
      return 0;
    case 'version':
    case '--version':
      c.stdout.write(`${VERSION}\n`);
      return 0;
    case 'list': {
      const o = parse(rest, ['json'], 1);
      const { status, ...result } = await lookup(c.providers, parseTask(o.args[0]));
      if (o.json) {
        json(result);
      } else {
        printList(result, c.stdout);
      }
      return status === 'complete' ? 0 : 1;
    }
    case 'open': {
      const o = parse(rest, ['provider', 'session'], 1);
      return open(c.providers, { provider: o.provider, sessionId: o.session, gid: parseTask(o.args[0]) }, { ...c.io, env: c.env });
    }
    case 'link': {
      const o = parse(rest, ['provider', 'session', 'task', 'replace']);
      json(await link(c.providers, { provider: o.provider, sessionId: o.session, gid: parseTask(o.task), replace: Boolean(o.replace) }, { env: c.env }));
      return 0;
    }
    case 'unlink': {
      const o = parse(rest, ['provider', 'session']);
      json(await unlink(c.providers, { provider: o.provider, sessionId: o.session }, { env: c.env }));
      return 0;
    }
    case 'doctor':
      parse(rest, []);
      return doctor(c);
    case 'asanamate':
      if (rest.length === 1 && rest[0] === 'list') {
        return asanamateList(c);
      }
      if (rest.length === 1 && rest[0] === 'open') {
        return asanamateOpen(c);
      }
      throw new UsageError('expected `asanamate list` or `asanamate open`');
    default:
      throw new UsageError(`unknown command ${JSON.stringify(display(command))}`);
  }
}

function printList({ conversations, diagnostics }, out) {
  if (!conversations.length) {
    out.write('No linked conversations.\n');
  }
  for (const r of conversations) {
    const when = r.updated_at ? r.updated_at.replace('T', ' ').slice(0, 16) : 'unknown time';
    out.write(`${display(r.provider)}  ${display(r.session_id)}  ${when}${r.archived ? '  archived' : ''}  activity: ${r.activity}\n`);
    out.write(`  ${display(r.title)}\n`);
    if (r.cwd || r.branch) {
      out.write(`  ${[r.cwd && `cwd: ${display(r.cwd)}`, r.branch && `branch: ${display(r.branch)}`].filter(Boolean).join('  ')}\n`);
    }
  }
  for (const d of diagnostics) {
    out.write(`${d.severity}: ${d.provider}: ${d.code}: ${d.message}\n`);
  }
}

// doctor checks runtimes, executables, histories, and metadata APIs without
// starting a model turn. Missing providers are reported, not failures.
async function doctor(c) {
  const run = promisify(execFile);
  const lines = [`ok: node ${process.versions.node}`];
  let failed = false;
  try {
    const sdk = JSON.parse(await readFile(join(dirname(fileURLToPath(import.meta.resolve('@anthropic-ai/claude-agent-sdk'))), 'package.json'), 'utf8'));
    lines.push(`ok: @anthropic-ai/claude-agent-sdk ${sdk.version}`);
  } catch {
    lines.push('error: @anthropic-ai/claude-agent-sdk is missing; reinstall asana-sessions');
    failed = true;
  }
  for (const name of ['claude', 'codex']) {
    try {
      const { stdout } = await run(name, ['--version'], { env: c.env, timeout: 5000 });
      lines.push(`ok: ${name} ${display(stdout)}`);
    } catch (err) {
      lines.push(err.code === 'ENOENT' ? `missing: ${name} is not on PATH` : `error: ${name} --version failed`);
      failed ||= err.code !== 'ENOENT';
    }
    try {
      await c.providers[name].probe();
      lines.push(`ok: ${name} history metadata is readable`);
    } catch (err) {
      const e = err instanceof ProviderError ? err : new ProviderError('provider_error', String(err.message));
      lines.push(`${e.code === 'unavailable' ? 'missing' : 'error'}: ${name}: ${display(e.message)}`);
      failed ||= e.code !== 'unavailable';
    }
  }
  c.stdout.write(`${lines.join('\n')}\n`);
  return failed ? 1 : 0;
}
