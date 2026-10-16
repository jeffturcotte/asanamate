#!/bin/sh
# Installs the packed tarball into an empty prefix with a fresh home and no
# checkout, asanamate, provider history, or Asana token, then checks that every
# public command runs and fails only for missing provider history.
set -eu
pkg=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tarball=$(cd "$pkg" && npm pack --silent --pack-destination "$work")
npm install --global --silent --no-audit --no-fund --prefix "$work/prefix" "$work/$tarball"

run() {
  (cd "$work" && env -i HOME="$work/home" PATH="$work/prefix/bin:$(dirname "$(command -v node)"):/usr/bin:/bin" asana-sessions "$@")
}
id=01a14609-3a63-7c02-bd2b-c313bedc3c37
mkdir "$work/home"

run version
run help > /dev/null
run doctor > "$work/doctor.txt" || { cat "$work/doctor.txt"; exit 1; }
grep -q '^ok: @anthropic-ai/claude-agent-sdk ' "$work/doctor.txt"
if run list https://app.asana.com/0/1/42 > "$work/list.txt"; then echo "list passed without history"; exit 1; fi
grep -q '^info: claude: unavailable' "$work/list.txt"
grep -q '^info: codex: unavailable' "$work/list.txt"
for cmd in "link --provider claude --session $id --task 42" "unlink --provider codex --session $id" "open 42 --provider codex --session $id"; do
  # shellcheck disable=SC2086
  if run $cmd 2> "$work/err.txt"; then echo "$cmd passed without history"; exit 1; fi
  grep -q 'not installed or has no history' "$work/err.txt"
done
echo "clean install ok"
