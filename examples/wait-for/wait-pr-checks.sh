#!/bin/sh
# Usage: wait-pr-checks.sh [PR]
# Waits for a pull request's checks (default: the current branch's PR), then notifies.
set -u
url=$(gh pr view ${1:+"$1"} --json url --jq .url) || exit 2

gh pr checks ${1:+"$1"} --watch --interval 30 >/dev/null 2>&1
failed=$(gh pr checks ${1:+"$1"} --json name,bucket --jq '[.[] | select(.bucket == "fail") | .name] | join(", ")') || exit 2

if [ -z "$failed" ]; then
    tell notify --type success --title "Checks passed" -- "$url" </dev/null
else
    tell notify --type failure --title "Checks failed" -- "$failed
$url" </dev/null
    exit 1
fi
