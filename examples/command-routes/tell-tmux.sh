#!/bin/sh
# Tell command-route provider: flash the message in tmux's status line for 5s.
set -u
tmux has-session 2>/dev/null || exit 3 # no tmux server: nothing sent
# tmux expands #{...} formats in messages; double every # to keep text literal.
body=$(tr '\n' ' ' | sed 's/#/##/g')
title=$(printf '%s' "${TELL_TITLE-}" | tr '\n' ' ' | sed 's/#/##/g')
exec tmux display-message -d 5000 "[$title] $body"
