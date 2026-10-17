#!/bin/sh
# Usage: watch-pr.sh [PR]
# Waits for a PR's checks (default: the current branch's PR). On success, tells
# you. On failure, asks the fixer agent to investigate, then tells you whether
# the agent was reached. The agent gets pointers and fetches logs itself, so no
# CI output passes through Tell.
set -u
pr_json=$(gh pr view ${1:+"$1"} --json url,headRefName,headRefOid) || exit 2
url=$(printf '%s' "$pr_json" | jq -r .url)
branch=$(printf '%s' "$pr_json" | jq -r .headRefName)
sha=$(printf '%s' "$pr_json" | jq -r '.headRefOid[0:7]')

gh pr checks "$url" --watch --interval 30 >/dev/null 2>&1
failed=$(gh pr checks "$url" --json name,bucket,link \
    --jq '.[] | select(.bucket == "fail") | "- \(.name): \(.link)"') || exit 2

if [ -z "$failed" ]; then
    tell notify --type success --title "CI passed: $branch" -- "$url" </dev/null || true
    exit 0
fi

tell send --intent request -- "CI failed on $branch at $sha ($url).
Failing checks:
$failed
Read logs with: gh run view <run-id> --log-failed
Fix on $branch and push. Do not rerun checks without a change." </dev/null
case $? in
    0) result="Asked the fixer agent to investigate." ;;
    3) result="Fixer agent not reached; nothing sent." ;;
    5) result="Agent dispatch uncertain; check before resending." ;;
    *) result="Agent dispatch failed; check before resending." ;;
esac
tell notify --type failure --title "CI failed: $branch" -- "$url
$result" </dev/null || true
exit 1
