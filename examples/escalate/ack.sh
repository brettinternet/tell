#!/bin/sh
# Usage: ack.sh INCIDENT_KEY
# Ends every escalation tier job raise.sh created for INCIDENT_KEY.
# A tier already delivered cannot be recalled; a tier running now is stopped.
set -u

[ "$#" -eq 1 ] || { echo 'usage: ack.sh INCIDENT_KEY' >&2; exit 2; }
key=$1

hash=$(printf '%s' "$key" | { sha256sum 2>/dev/null || shasum -a 256; } | cut -c1-12)
prefix=inc-$hash

jobs=$(noon --user --json list </dev/null | jq -r --arg p "$prefix-t" \
    '.jobs[] | select(.name | startswith($p)) | select(.ended | not) | .name') || exit 1

if [ -z "$jobs" ]; then
    echo "ack: no active tiers for $key ($prefix-*)" >&2
    exit 0
fi

# shellcheck disable=SC2086 # job names never contain spaces
noon --user cancel $jobs </dev/null
