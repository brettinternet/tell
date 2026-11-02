#!/bin/sh
# Usage: raise.sh INCIDENT_KEY TITLE
# Schedules each later tier as a Noon temporary job, then sends tier 1 as a
# Pushover emergency alert tagged with the incident and saves its receipt.
# Raising the same incident again changes nothing until ack.sh ends it.
set -u

# Policy; the environment may override each value. Tier N>1 is
# "DELAY [tell notify options]" (split on spaces) and runs once, DELAY after
# raise, unless tier 1 was acknowledged in Pushover. Set a tier empty to drop it.
: "${CRITICAL=$HOME/.config/tell/critical.yaml}"  # tier 1 config: pushover emergency
: "${TIER1_NOTIFY=--type failure}"
: "${TIER2=10m --type failure}"
: "${TIER3=30m --type failure --via apprise --apprise-tag phone}"
: "${LIFETIME=2h}"  # unrun tiers end on their own after this (Noon max 7d)
: "${PUSHOVER_CALLBACK=}"  # optional URL Pushover calls on acknowledgement
: "${STATE=${XDG_STATE_HOME:-$HOME/.local/state}/tell-escalate}"

[ "$#" -eq 2 ] || { echo 'usage: raise.sh INCIDENT_KEY TITLE' >&2; exit 2; }
key=$1 title=$2

hash=$(printf '%s' "$key" | { sha256sum 2>/dev/null || shasum -a 256; } | cut -c1-12)
prefix=inc-$hash
here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# Each raise gets its own receipt file, so a stale receipt from an earlier
# raise can never make this raise's tiers skip themselves.
receipt_file=$STATE/$prefix.$(date +%s).$$.receipt

created=
tier=2
for policy in "${TIER2:-}" "${TIER3:-}"; do
    if [ -n "$policy" ]; then
        # shellcheck disable=SC2086 # policy is space-separated words
        set -- $policy
        delay=$1
        shift
        result=$(noon --user --json add "$prefix-t$tier" --temporary \
            --key "$prefix" --in "$delay" --for "$LIFETIME" --timeout 30s \
            --description "Escalation tier $tier for $key" \
            -- "$here/tier.sh" "$CRITICAL" "$receipt_file" \
            tell --timeout 10s notify --title "$title" "$@" \
            -- "Incident $key, tier $tier. Acknowledge: ack.sh '$key'" </dev/null) || {
            printf 'raise: scheduling tier %s failed: %s\n' "$tier" \
                "$(printf '%s' "$result" | jq -r '.error.message' 2>/dev/null)" >&2
            exit 1
        }
        if [ "$(printf '%s' "$result" | jq -r '.created')" = true ]; then
            created=yes
        else
            created=${created:-no}
        fi
    fi
    tier=$((tier + 1))
done

if [ "$created" = no ]; then
    echo "raise: $key is already raised ($prefix-*); nothing sent" >&2
    exit 0
fi

mkdir -p "$STATE" || exit 1

# shellcheck disable=SC2086 # policy options are space-separated words
result=$(tell --config "$CRITICAL" --json notify --title "$title" $TIER1_NOTIFY \
    --pushover-tag "$prefix" ${PUSHOVER_CALLBACK:+--pushover-callback "$PUSHOVER_CALLBACK"} \
    -- "Incident $key, tier 1. Acknowledge: ack.sh '$key'" </dev/null)
status=$?
printf '%s' "$result" | jq -r '.receipt // empty' >"$receipt_file"
exit "$status"
