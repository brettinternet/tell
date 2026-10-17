#!/bin/sh
# Usage: fresher-than.sh MINUTES PATH   exit 0 if PATH (or anything under it) changed recently.
# Example: a backup directory that should receive a snapshot every night.
set -u
[ -e "$2" ] || exit 1
find "$2" -mmin "-$1" -print 2>/dev/null | head -n 1 | grep -q .
