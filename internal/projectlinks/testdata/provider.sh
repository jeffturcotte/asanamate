#!/bin/sh
# Minimal project links provider for the protocol test (docs/extensions.md).
# It appends each request to the file named by $1, then answers it.
request=$(cat)
printf '%s\n' "$request" >> "$1"
case $request in
*'"project_gid":"7"'*)
	printf '%s\n' '{"links":[{"type":"git","value":"https://github.com/acme/web"},{"type":"harvest_project","value":"123"}]}'
	;;
*'"operation":"links"'*)
	printf '%s\n' '{"links":[]}'
	;;
*)
	echo "unknown operation" >&2
	exit 1
	;;
esac
