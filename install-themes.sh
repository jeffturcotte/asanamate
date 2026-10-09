#!/bin/sh
# Install editable standard themes from a release archive or source checkout.
set -eu
umask 077

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
theme_dir="${XDG_CONFIG_HOME:-$HOME/.config}/asanamate/themes"
mkdir -p "$theme_dir"

installed=0
kept=0
for source in "$source_dir"/themes/*.toml "$source_dir"/themes/LICENSE*; do
    if [ ! -f "$source" ]; then
        printf 'Missing theme files in %s/themes\n' "$source_dir" >&2
        exit 1
    fi
    target="$theme_dir/${source##*/}"
    if [ -e "$target" ] || [ -L "$target" ]; then
        kept=$((kept + 1))
        continue
    fi
    # Refuse replacement even if another process creates the file meanwhile.
    (set -C; cat "$source" > "$target")
    installed=$((installed + 1))
done
printf 'Theme files: installed %s, kept %s in %s\n' "$installed" "$kept" "$theme_dir"
