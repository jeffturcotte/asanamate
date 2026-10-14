#!/bin/sh
# Install every asanamate command from this checkout with go install, then
# add any new standard themes. Run it by hand; nothing else calls it.
set -eu

cd "$(dirname -- "$0")"
go install ./cmd/...
sh ./install-themes.sh
printf 'Installed %s:\n' "$(git describe --always --dirty 2>/dev/null || echo 'this checkout')"
go list -f '  {{.Target}}' ./cmd/...
