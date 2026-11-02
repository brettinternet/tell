#!/bin/sh
# Usage: tier.sh CONFIG RECEIPT_FILE COMMAND...
# Runs one later escalation tier unless the tier-1 Pushover alert was already
# acknowledged in the app. Any doubt (no receipt, Pushover unreachable) sends.
set -u

[ "$#" -ge 3 ] || { echo 'usage: tier.sh CONFIG RECEIPT_FILE COMMAND...' >&2; exit 2; }
config=$1 file=$2
shift 2

receipt=$(cat "$file" 2>/dev/null) || receipt=
if [ -n "$receipt" ] && tell --config "$config" pushover status "$receipt" >/dev/null </dev/null; then
    echo 'tier: acknowledged in Pushover; not sent' >&2
    exit 0
fi
exec "$@"
