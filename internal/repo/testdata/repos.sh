#!/bin/sh
# Minimal repo source for the protocol test (docs/extensions.md): one repo
# path per line. asanamate expands a leading ~ and skips blanks and repeats.
printf '%s\n' '~/code/web' "$1" '' "  $1  "
