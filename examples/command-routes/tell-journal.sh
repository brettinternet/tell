#!/bin/sh
# Tell command-route provider: append one tab-separated line per notification.
# Useful as an audit trail of what your automations and agents said:
#   tail -f ~/.local/state/tell/journal.tsv
set -u
log=${XDG_STATE_HOME:-$HOME/.local/state}/tell/journal.tsv
mkdir -p "$(dirname "$log")" || exit 1
body=$(tr '\n\t' '  ')
title=$(printf '%s' "${TELL_TITLE-}" | tr '\n\t' '  ')
printf '%s\t%s\t%s\t%s\t%s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" \
    "${TELL_TYPE-info}" "${TELL_PROJECT-}" "$title" "$body" >>"$log"
