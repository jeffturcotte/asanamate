#!/bin/sh
# Minimal feed for the protocol test (docs/extensions.md). It appends each
# request to the file named by $1, then answers it.
request=$(cat)
printf '%s\n' "$request" >> "$1"
case $request in
*'"operation":"feed"'*)
	printf '%s\n' '{"source":"ci","summary":"1 failed","items":[
	  {"time":"2026-10-12T14:05:00-04:00","text":"build failed","branch":"feature/x","repo":"/code/web","tone":"error"},
	  {"time":"2026-10-12T15:30:00-04:00","text":"deploy window","url":"https://example.com/deploys"}]}'
	;;
*)
	echo "unknown operation" >&2
	exit 1
	;;
esac
