#!/bin/sh
# Usage: wait-url.sh URL [TIMEOUT_SECONDS]
# Polls URL until it answers 2xx/3xx, then notifies. Notifies failure on timeout.
set -u
url=$1
deadline=$(($(date +%s) + ${2:-300}))
start=$(date +%s)

until curl -fsS -o /dev/null --max-time 5 "$url" 2>/dev/null; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
        tell notify --type failure --title "Not ready" -- "$url did not respond within ${2:-300}s" </dev/null
        exit 1
    fi
    sleep 2
done
tell notify --type success --title "Ready" -- "$url is up after $(($(date +%s) - start))s" </dev/null
