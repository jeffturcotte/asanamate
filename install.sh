#!/bin/sh
# Install every asanamate command from this checkout with go install, after
# backing up the binaries it replaces, then add any new standard themes.
# Run it by hand; nothing else calls it.
set -eu

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$source_dir"

bin_dir=$(go env GOBIN)
if [ -z "$bin_dir" ]; then
    gopath=$(go env GOPATH)
    bin_dir="${gopath%%:*}/bin"
fi
stamp=$(date +%Y-%m-%d--%H-%M-%S)

backed_up=0
for dir in cmd/*/; do
    binary="$bin_dir/$(basename "$dir")"
    if [ -f "$binary" ]; then
        cp -p "$binary" "$binary.$stamp.bak"
        backed_up=$((backed_up + 1))
    fi
done

go install ./cmd/...
sh ./install-themes.sh

revision=$(git describe --always --dirty 2>/dev/null || echo unknown)
printf 'Installed %s commands at %s in %s (%s backed up as *.%s.bak)\n' \
    "$(ls -d cmd/*/ | wc -l | tr -d ' ')" "$revision" "$bin_dir" "$backed_up" "$stamp"
