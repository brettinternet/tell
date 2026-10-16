#!/bin/sh
# Repo-owned Noon hook; Tell remains scheduler-neutral. See README.md.
# Missing/pruned history is not proof of a transition: establish a baseline.
case "${NOON_PREVIOUS_STATUS-}:${NOON_STATUS-}" in
    succeeded:failed|succeeded:timed_out) kind=failure ;;
    failed:succeeded|timed_out:succeeded) kind=success ;;
    *) exit 0 ;;
esac

if [ -z "${NOON_PROJECT-}" ] || [ -z "${NOON_JOB-}" ] || [ -z "${NOON_RUN_ID-}" ]; then
    printf '%s\n' 'Noon notification skipped: missing hook context.' >&2
    exit 0
fi

# Never read hook stdin or NOON_LOG_FILE. Pass a literal message, not '-',
# so the JSON run summary cannot become a notification. No log excerpts.
message="Job: $NOON_JOB
Status: $NOON_STATUS
Run: $NOON_RUN_ID
Project: $NOON_PROJECT
Logs: run noon logs $NOON_RUN_ID from the project directory."

# Best effort: never retry delivery or rerun the business job. Keep Tell's
# timeout below Noon's 30-second hook budget, including startup and cleanup.
tell -C "$NOON_PROJECT" --timeout 10s notify \
    --title 'Noon job transition' --type "$kind" -- "$message" </dev/null || true
