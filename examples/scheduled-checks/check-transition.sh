#!/bin/sh
# Usage: check-transition.sh NAME CMD [ARG...]
# Runs CMD (exit 0 = healthy) and notifies only when NAME starts failing or
# recovers. A check that fails on its very first run also notifies.
# State is one small file per NAME; Tell itself keeps no state.
set -u
name=${1-}
case $name in
    '' | *[!A-Za-z0-9_.-]*)
        printf '%s\n' 'check-transition: NAME must match [A-Za-z0-9_.-]+' >&2
        exit 2
        ;;
esac
shift
state_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tell-checks
mkdir -p "$state_dir" || exit 1
state_file=$state_dir/$name

if "$@" >/dev/null 2>&1; then now=ok; else now=failing; fi
before=$(cat "$state_file" 2>/dev/null || echo unknown)

case $before:$now in
    ok:failing | unknown:failing)
        type=failure message="$name is failing on $(hostname). Check: $*" ;;
    failing:ok)
        type=success message="$name recovered on $(hostname)." ;;
    *)
        printf '%s\n' "$now" >"$state_file"
        exit 0
        ;;
esac

tell --timeout 10s notify --type "$type" --title "Check: $name" -- "$message" </dev/null
case $? in
    # config, usage, not_sent: nothing was delivered. Keep the old state so the
    # next run reports this transition again instead of losing it.
    1 | 2 | 3) exit 0 ;;
esac
# accepted, failed, or uncertain: it may have arrived. Never resend blindly.
printf '%s\n' "$now" >"$state_file"
