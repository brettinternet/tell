#!/bin/sh
# Usage: request-review.sh [BASE]
# Run from the implementer's checkout after committing. Asks the reviewer agent
# (send.to, or TELL_SEND_TO) to review the branch. Sends pointers only: the
# reviewer reads the diff itself, from the exact commit.
set -eu
base=${1:-main}
if [ -n "$(git status --porcelain)" ]; then
    printf '%s\n' 'request-review: commit or stash first; the reviewer sees commits only' >&2
    exit 2
fi
branch=$(git symbolic-ref --short HEAD)
sha=$(git rev-parse --short HEAD)
top=$(git rev-parse --show-toplevel)
files=$(git diff --name-only "$base...HEAD" | wc -l | tr -d ' ')
if [ "$files" -eq 0 ]; then
    printf 'request-review: no changes against %s\n' "$base" >&2
    exit 2
fi

exec tell send --intent request -- "Review $branch at $sha against $base ($files files).
Read: git -C $top diff $base...$sha
Report blockers first, then suggestions. Do not edit that checkout."
