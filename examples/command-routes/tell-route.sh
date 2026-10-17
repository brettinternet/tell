#!/bin/sh
# Tell command-route provider: a personal router.
#   failure, warning  → phone (tell-ntfy.sh), any time
#   info, success     → desktop banner, except during quiet hours (22:00–07:00),
#                       when it exits 3 so Tell reports not_sent instead of pretending.
set -u
here=$(dirname "$0")

case ${TELL_TYPE-info} in
    failure | warning) exec "$here/tell-ntfy.sh" ;;
esac

hour=$(date +%H)
if [ "$hour" -ge 22 ] || [ "$hour" -lt 7 ]; then
    exit 3
fi

# Explicit --via prevents recursion into this router. A shorter timeout keeps the
# inner delivery inside the outer one; inner exit 3 maps to not_sent outside too.
exec tell -C "${TELL_PROJECT:-.}" --via desktop --timeout 5s \
    notify --title "${TELL_TITLE-}" --type "${TELL_TYPE-info}" -
