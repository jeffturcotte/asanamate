import { UsageError } from './errors.js';

// Shared Asana ticket rules: task URLs, the canonical title marker, and Claude's
// recognized tag. Provider modules never parse titles themselves.

const DIGITS = /^[0-9]+$/;
const TRAILING_MARKER = /\s*\[asana:([0-9]+)\]$/u;
const TAG = /^asana:([0-9]+)$/;

export const DEFAULT_PREFIX = 'Conversation';

export function validGID(value) {
  return typeof value === 'string' && DIGITS.test(value);
}

// taskGIDFromURL accepts only HTTPS task links on app.asana.com, in the legacy
// /0/<project>/<task>[/f] form or a modern /1/<workspace>/.../task/<task>[/f]
// form. Query strings and fragments do not change identity. Returns '' otherwise.
export function taskGIDFromURL(raw) {
  let url;
  try {
    url = new URL(String(raw).trim());
  } catch {
    return '';
  }
  if (url.protocol !== 'https:' || url.hostname !== 'app.asana.com' || url.port || url.username || url.password) {
    return '';
  }
  const parts = url.pathname.replace(/^\/+|\/+$/g, '').split('/');
  const tail = (i) => parts.length === i + 1 || (parts.length === i + 2 && parts[i + 1] === 'f');
  if (parts[0] === '0') {
    return validGID(parts[1]) && validGID(parts[2]) && tail(2) ? parts[2] : '';
  }
  if (parts[0] === '1' && validGID(parts[1])) {
    const i = parts.indexOf('task');
    if (i > 1 && i === parts.lastIndexOf('task') && validGID(parts[i + 1]) && tail(i + 1)) {
      return parts[i + 1];
    }
  }
  return '';
}

// URL_CANDIDATE finds app.asana.com links in free text, stopping at whitespace,
// quotes, and brackets so Markdown and angle-bracket links still parse.
const URL_CANDIDATE = /https:\/\/app\.asana\.com\/[^\s<>"'`()[\]{}]+/gi;

// taskGIDsInText returns the distinct task GIDs linked in text, in order.
// Trailing sentence punctuation is not part of a link.
export function taskGIDsInText(text) {
  const gids = new Set();
  for (const [raw] of String(text ?? '').matchAll(URL_CANDIDATE)) {
    const gid = taskGIDFromURL(raw.replace(/[.,;:!?]+$/, ''));
    if (gid) {
      gids.add(gid);
    }
  }
  return [...gids];
}

// parseTask accepts a task URL or, for explicit commands only, a raw GID.
export function parseTask(input) {
  const value = String(input ?? '').trim();
  const gid = validGID(value) ? value : taskGIDFromURL(value);
  if (!gid) {
    throw new UsageError(`not an Asana task URL or GID: ${JSON.stringify(value).slice(0, 200)}`);
  }
  return gid;
}

// splitTitle separates a title's human-readable prefix from its trailing markers,
// in title order. A marker inside the prefix is ordinary text.
export function splitTitle(title) {
  let prefix = title ?? '';
  const gids = [];
  for (let m = prefix.match(TRAILING_MARKER); m; m = prefix.match(TRAILING_MARKER)) {
    gids.unshift(m[1]);
    prefix = prefix.slice(0, m.index);
  }
  return { prefix: prefix.trim(), gids };
}

export function tagGID(tag) {
  return tag?.match(TAG)?.[1] ?? '';
}

// ticketLink reports the GID a native title (and optional Claude tag) links to.
// Disagreeing markers or tag make a conflict, which never matches automatically.
export function ticketLink(title, tag) {
  const { gids } = splitTitle(title);
  const found = new Set(gids);
  const fromTag = tagGID(tag);
  if (fromTag) {
    found.add(fromTag);
  }
  if (found.size > 1) {
    return { gid: '', conflict: [...found] };
  }
  return { gid: [...found][0] ?? '', conflict: [] };
}

// linkTitle appends one canonical marker to prefix. With a limit (code points),
// only the prefix is trimmed, at a grapheme boundary, so the marker stays whole.
export function linkTitle(prefix, gid, limit) {
  const marker = ` [asana:${gid}]`;
  let text = (prefix ?? '').trim() || DEFAULT_PREFIX;
  if (limit) {
    const room = limit - [...marker].length;
    if (room < 1) {
      throw new Error(`title limit ${limit} cannot hold the ticket marker`);
    }
    let kept = '';
    for (const { segment } of new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text)) {
      if ([...kept].length + [...segment].length > room) {
        break;
      }
      kept += segment;
    }
    text = kept.trimEnd() || [...DEFAULT_PREFIX].slice(0, room).join('');
  }
  return text + marker;
}
