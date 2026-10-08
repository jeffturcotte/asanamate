#!/bin/sh
# Minimal agents source for the protocol test (docs/extensions.md): one
# "<path>\t<status>[\t<target>[\t<title>[\t<branch>\t<repo>]]]" line per agent
# running in $1.
printf '%s\t%s\t%s\t%s\n' "$1" busy s1 'Fix login'
printf '%s\t%s\t%s\n' "$1" permission s2
printf '%s\t%s\n' "$1" mystery
printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$1" idle s3 '' fix-login "$1"
echo 'lines without a tab are skipped'
