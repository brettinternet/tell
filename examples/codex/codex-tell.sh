#!/bin/sh
# Codex `notify` program. Codex appends one JSON argument per event; only
# agent-turn-complete is handled. The assistant's message is not forwarded.
set -u
payload=${1-}
[ "$(printf '%s' "$payload" | jq -r '.type // empty' 2>/dev/null)" = agent-turn-complete ] || exit 0

cwd=$(printf '%s' "$payload" | jq -r '.cwd // empty' 2>/dev/null)
[ -d "$cwd" ] || cwd=$PWD

tell -C "$cwd" --timeout 5s notify --type success \
    --title "Codex · $(basename "$cwd")" -- "Turn complete; waiting for your next prompt" \
    </dev/null >/dev/null 2>&1
exit 0
