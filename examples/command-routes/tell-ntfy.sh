#!/bin/sh
# Tell command-route provider: push the notification to your phone with ntfy.
# The topic URL and optional auth header live in ntfy's own files, not Tell config:
#   ~/.config/tell-ntfy/url      https://ntfy.sh/a-long-random-topic-name
#   ~/.config/tell-ntfy/headers  Authorization: Bearer <token>   (optional, chmod 600)
set -u
dir=${XDG_CONFIG_HOME:-$HOME/.config}/tell-ntfy
[ -r "$dir/url" ] || exit 3 # exit 3 promises nothing was sent: Tell reports not_sent

case ${TELL_TYPE-info} in
    failure) priority=urgent tags=rotating_light ;;
    warning) priority=high tags=warning ;;
    success) priority=default tags=white_check_mark ;;
    *) priority=default tags=speech_balloon ;;
esac
# Headers are single-line; the body (stdin) may be multi-line.
title=$(printf '%s' "${TELL_TITLE-}" | tr '\n' ' ')

set -- -fsS -o /dev/null --max-time 8 \
    -H "Title: $title" -H "Priority: $priority" -H "Tags: $tags" \
    --data-binary @- "$(cat "$dir/url")"
# curl reads the header file itself, so the token never appears in argv.
[ -r "$dir/headers" ] && set -- -H "@$dir/headers" "$@"
exec curl "$@"
