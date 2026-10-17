#!/bin/sh
# Claude Code hook for UserPromptSubmit, Notification, and Stop events.
#   Notification: Claude is waiting on a permission prompt or your input → always notify.
#   Stop: a turn finished → notify only if it ran at least TELL_CLAUDE_MIN_SECONDS (default 60).
# Prints nothing on stdout: UserPromptSubmit stdout would be added to Claude's context.
# Forwards only the event, project name, and Claude Code's own notification text,
# never the transcript or tool output.
set -u
input=$(cat)
field() { printf '%s' "$input" | jq -r --arg k "$1" '.[$k] // empty' 2>/dev/null; }

event=$(field hook_event_name)
session=$(field session_id)
cwd=$(field cwd)
[ -d "$cwd" ] || cwd=$PWD
case $session in '' | *[!A-Za-z0-9_-]*) session=unknown ;; esac
started=${TMPDIR:-/tmp}/claude-tell-$session

case $event in
    UserPromptSubmit)
        date +%s >"$started"
        exit 0
        ;;
    Notification)
        type=warning body=$(field message)
        body=${body:-Claude Code needs your attention}
        ;;
    Stop)
        start=$(cat "$started" 2>/dev/null)
        case $start in '' | *[!0-9]*) exit 0 ;; esac
        elapsed=$(($(date +%s) - start))
        [ "$elapsed" -ge "${TELL_CLAUDE_MIN_SECONDS:-60}" ] || exit 0
        type=success body="Finished after ${elapsed}s; waiting for your next prompt"
        ;;
    *) exit 0 ;;
esac

tell -C "$cwd" --timeout 5s notify --type "$type" \
    --title "Claude Code · $(basename "$cwd")" -- "$body" </dev/null >/dev/null 2>&1
exit 0
