---
name: link-asana-task
description: Link this conversation to an Asana task, change its linked task, or unlink it using asana-sessions.
---

Use the separately installed `asana-sessions` helper. No Asana API or token is needed.

1. Use the exact task URL or numeric GID supplied by the user.
2. Run `asana-sessions link --provider codex --current --task <task-url-or-gid>`.
   `--current` validates the host's CODEX_THREAD_ID environment variable. If unavailable,
   explain the limitation and require an explicit session ID; use `--session <id>`.
   Never guess from the newest conversation, cwd, transcript search, or parent context.
3. If another task is linked, report the conflict. Add `--replace` only when the
   user explicitly requests changing the linked task. That request is sufficient authorization.
4. To unlink, run `asana-sessions unlink --provider codex --current`
   (or `--session <id>` when explicitly supplied).
5. Report the helper's verified result. A failed command is not a successful link.

Preserve the human title and unrelated Claude tags through the helper. Never
substitute `/rename`, edit transcripts, or modify user skills or instruction files.
