#!/bin/sh
# Fails if two files would collide on a case-insensitive filesystem (macOS,
# Windows), including module imports that resolve without their .ts/.js
# extension, such as Toast.svelte and toast.svelte.ts.
set -eu
cd "$(dirname -- "$0")/.."
dupes=$(git ls-files --cached --others --exclude-standard |
	sed -E 's/\.(ts|js)$//' |
	tr '[:upper:]' '[:lower:]' |
	sort | uniq -d)
if [ -n "$dupes" ]; then
	echo "These paths collide on case-insensitive filesystems:" >&2
	echo "$dupes" >&2
	exit 1
fi
