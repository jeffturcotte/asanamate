// display makes untrusted provider text safe for one terminal line: escape
// sequences, controls, and format characters become spaces. Never apply it to
// IDs or paths passed to a provider.
const ESCAPES = /\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-_]/g;
const CONTROLS = /[\p{Cc}\p{Cf}]/gu;

export function display(value) {
  return String(value ?? '').replace(ESCAPES, ' ').replace(CONTROLS, ' ').replace(/\s+/g, ' ').trim();
}
