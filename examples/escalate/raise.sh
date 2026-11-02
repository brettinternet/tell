#!/bin/sh
# Usage: raise.sh INCIDENT_KEY TITLE
# Sends tier 1 now and schedules each later tier as a Noon temporary job.
# Raising the same incident again changes nothing until ack.sh ends it.
set -u

# Policy; the environment may override each value. Tier N>1 is
# "DELAY [tell notify options]" (split on spaces) and runs once, DELAY after
# raise. Set a tier empty to drop it.
: "${TIER1_NOTIFY=--type warning}"
: "${TIER2=10m --type failure}"
: "${TIER3=30m --type failure --via apprise --apprise-tag phone}"
: "${LIFETIME=2h}"  # unrun tiers end on their own after this (Noon max 7d)

[ "$#" -eq 2 ] || { echo 'usage: raise.sh INCIDENT_KEY TITLE' >&2; exit 2; }
key=$1 title=$2

hash=$(printf '%s' "$key" | { sha256sum 2>/dev/null || shasum -a 256; } | cut -c1-12)
prefix=inc-$hash

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
            -- tell --timeout 10s notify --title "$title" "$@" \
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

# shellcheck disable=SC2086 # policy options are space-separated words
tell notify --title "$title" $TIER1_NOTIFY \
    -- "Incident $key, tier 1. Acknowledge: ack.sh '$key'" </dev/null
