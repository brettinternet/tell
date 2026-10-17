#!/bin/sh
# Tell command-route provider: speak failures aloud on macOS; stay silent otherwise.
[ "${TELL_TYPE-}" = failure ] || exit 3 # nothing sent
exec /usr/bin/say -f -
