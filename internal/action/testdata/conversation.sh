#!/bin/sh
# The foreground transport leaves stdin attached to the terminal.
if [ -t 0 ]; then printf '%s\n' 'terminal=yes'; fi
IFS= read -r input || exit 1
printf 'input=%s\nprovider=%s\nid=%s\nsnapshot=%s\n' "$input" \
  "$ASANAMATE_CONVERSATION_PROVIDER" "$ASANAMATE_CONVERSATION_ID" \
  "$ASANAMATE_CONVERSATION_JSON"
[ "$input" != fail ]
