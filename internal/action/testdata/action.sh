#!/bin/sh
# Minimal action for the protocol test (docs/extensions.md): it reads the
# ticket from ASANAMATE_* variables and files, then exits with $1 (default 0).
printf 'gid=%s\n' "$ASANAMATE_GID"
printf 'branch=%s\n' "$ASANAMATE_BRANCH"
printf 'project=%s\n' "$ASANAMATE_PROJECT_GID"
printf 'dir=%s\n' "$(pwd -P)"
head -n 1 "$ASANAMATE_TICKET_MD"
exit "${1:-0}"
