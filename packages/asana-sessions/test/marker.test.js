import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { UsageError } from '../src/errors.js';
import { linkTitle, parseTask, splitTitle, taskGIDFromURL, ticketLink } from '../src/marker.js';

const fixtures = JSON.parse(await readFile(new URL('./fixtures/task-urls.json', import.meta.url), 'utf8'));

test('task URL fixtures', () => {
  for (const { url, gid } of fixtures) {
    assert.equal(taskGIDFromURL(url), gid, url);
  }
});

test('parseTask accepts a raw GID only as an explicit argument', () => {
  assert.equal(parseTask('1234567890'), '1234567890');
  assert.equal(parseTask('https://app.asana.com/0/1/77'), '77');
  assert.throws(() => parseTask('task 77'), UsageError);
  assert.throws(() => parseTask(''), UsageError);
});

test('ticketLink matches exact trailing markers only', () => {
  assert.deepEqual(ticketLink('Fix sorting [asana:42]'), { gid: '42', conflict: [] });
  assert.deepEqual(ticketLink('Fix [asana:42] sorting'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink('Fix sorting [asana:42]x'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink('Fix sorting [ASANA:42]'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink('Fix sorting [asana:4a2]'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink('Fix 1234567890'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink(null), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink('A [asana:42] [asana:42]'), { gid: '42', conflict: [] });
  assert.deepEqual(ticketLink('A [asana:42] [asana:7]'), { gid: '', conflict: ['42', '7'] });
});

test('ticketLink recognizes only an exact Claude tag', () => {
  assert.deepEqual(ticketLink(null, 'asana:42'), { gid: '42', conflict: [] });
  assert.deepEqual(ticketLink('A [asana:42]', 'asana:42'), { gid: '42', conflict: [] });
  assert.deepEqual(ticketLink('A [asana:42]', 'asana:7'), { gid: '', conflict: ['42', '7'] });
  assert.deepEqual(ticketLink(null, 'asana:42 extra'), { gid: '', conflict: [] });
  assert.deepEqual(ticketLink(null, 'favorite'), { gid: '', conflict: [] });
});

test('splitTitle keeps the prefix and drops every trailing marker', () => {
  assert.deepEqual(splitTitle('Fix sorting [asana:42] [asana:42]'), { prefix: 'Fix sorting', gids: ['42', '42'] });
  assert.deepEqual(splitTitle('[asana:42]'), { prefix: '', gids: ['42'] });
});

test('linkTitle trims only the prefix at a grapheme boundary', () => {
  assert.equal(linkTitle('Fix sorting', '42'), 'Fix sorting [asana:42]');
  assert.equal(linkTitle('', '42'), 'Conversation [asana:42]');
  assert.equal(linkTitle(null, '42'), 'Conversation [asana:42]');
  assert.equal(linkTitle('abcdef', '42', 14), 'abc [asana:42]');
  // The family emoji is one grapheme of several code points: never split.
  const family = '👨‍👩‍👧';
  const title = linkTitle(`${family}${family}`, '42', 11 + [...family].length + 1);
  assert.equal(title, `${family} [asana:42]`);
  assert.throws(() => linkTitle('x', '42', 5));
});
