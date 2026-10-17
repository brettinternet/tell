#!/bin/sh
# Usage: wait-pid.sh PID
# Notifies when a process you already started exits. Its exit status is not
# observable from outside, so this only reports that it ended.
set -u
pid=$1
name=$(ps -p "$pid" -o comm= 2>/dev/null) || {
    printf 'wait-pid: no process %s\n' "$pid" >&2
    exit 2
}
while kill -0 "$pid" 2>/dev/null; do
    sleep 5
done
tell notify --title "Process exited" -- "$(basename "$name") (pid $pid) has exited" </dev/null
