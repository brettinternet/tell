#!/usr/bin/env bash
# Usage: summarize-failure.sh CMD [ARG...]
# Runs CMD with output shown and saved. On failure, a local model reads the
# log tail and the notification carries a one-sentence diagnosis plus the log path.
# The log never leaves the machine; only the model's summary is delivered.
set -uo pipefail
log_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tell-logs
mkdir -p "$log_dir" || exit 1
log=$(mktemp "$log_dir/$(basename "$1").XXXXXX") || exit 1

"$@" 2>&1 | tee "$log"
status=$?

if [ "$status" -eq 0 ]; then
    tell notify --type success --title "Done: $1" -- "$* finished" </dev/null
    exit 0
fi

prompt='Below is the end of a failed command output. In one sentence of at most 30 words, state the most likely root cause. No preamble, no markdown.'
summary=$(tail -n 200 "$log" | ollama run "${OLLAMA_MODEL:-llama3.2}" "$prompt" 2>/dev/null | head -c 400)
tell notify --type failure --title "Failed: $1" -- "${summary:-No summary (is ollama running?)}
Exit $status. Log: $log" </dev/null
exit "$status"
