#!/bin/sh
# Usage: report-tests.sh CMD [ARG...]   (usually run by watchexec on file changes)
# Runs the test command with its output visible in your pane, then informs the
# coding agent only when the suite flips between passing and failing.
set -u
key=$(printf '%s' "$PWD" | cksum | cut -d ' ' -f 1)
state=${TMPDIR:-/tmp}/tell-test-watch-$key

if "$@"; then now=pass; else now=fail; fi
before=$(cat "$state" 2>/dev/null || echo unknown)

case $before:$now in
    pass:fail | unknown:fail)
        message="Tests started failing after recent edits in $PWD. Reproduce: $*" ;;
    fail:pass)
        message="Tests pass again in $PWD ($*)." ;;
    *)
        printf '%s\n' "$now" >"$state"
        exit 0
        ;;
esac

tell send --intent inform -- "$message" </dev/null
# Nothing sent (1-3): keep the old state so the next run reports the flip again.
case $? in 1 | 2 | 3) exit 0 ;; esac
printf '%s\n' "$now" >"$state"
