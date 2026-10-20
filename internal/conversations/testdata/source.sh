#!/bin/sh
# Fixture source: the paths are supplied by the test, never ticket metadata.
cat > "$REQUEST_FILE" || exit 1
cat "$RESPONSE_FILE"
