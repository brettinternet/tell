#!/bin/sh
# Usage: deliver.sh MESSAGE
# Hands a request to an agent, trying in order:
#   1. its Herdr pane, typed as a new turn only if the agent is idle
#   2. its hcom mailbox, read at its next hook
#   3. you, as a notification that no agent was reached
# Moves on only after not_sent (exit 3): nothing was delivered, so trying
# elsewhere cannot duplicate it. Any other outcome stops the chain.
set -u
message=$1

try_send() {
    route=$1 recipient=$2
    result=$(tell --json --send-via "$route" --to "$recipient" \
        send --intent request -- "$message" </dev/null 2>/dev/null)
    code=$?
    reason=$(printf '%s' "$result" | jq -r '.error.reason // .outcome' 2>/dev/null)
    printf '%s %s: %s\n' "$route" "$recipient" "${reason:-unknown}" >&2
    return "$code"
}

try_send herdr "${HERDR_AGENT:-reviewer}"
code=$?
[ "$code" -eq 3 ] || exit "$code"

try_send hcom "${HCOM_AGENT:-reviewer}"
code=$?
[ "$code" -eq 3 ] || exit "$code"

tell notify --type warning --title "No agent reached" -- "Not delivered to any agent:
$message" </dev/null
exit 3
