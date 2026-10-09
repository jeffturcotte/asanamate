#!/bin/sh
# Minimal time tracking provider for the protocol test (docs/extensions.md).
# It appends each request to the file named by $1, then answers it.
request=$(cat)
printf '%s\n' "$request" >> "$1"
case $request in
*'"operation":"form"'*)
	printf '%s\n' '{"fields":[{"id":"project_id","label":"Project","type":"select","remember":true,"options":[{"id":"123","name":"Web"}]},{"id":"hours","label":"Hours","type":"hours"},{"id":"spent_date","label":"Date","type":"date"}]}'
	;;
*'"operation":"log"'*)
	;;
*'"operation":"summary"'*)
	printf '%s\n' '{"total_seconds":13500}'
	;;
*)
	echo "unknown operation" >&2
	exit 1
	;;
esac
