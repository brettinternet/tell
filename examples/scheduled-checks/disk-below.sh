#!/bin/sh
# Usage: disk-below.sh PERCENT [MOUNT]   exit 0 while MOUNT (default /) is below PERCENT used.
set -u
used=$(df -P "${2:-/}" | awk 'NR == 2 { sub(/%/, "", $5); print $5 }')
[ -n "$used" ] && [ "$used" -lt "$1" ]
