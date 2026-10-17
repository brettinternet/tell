#!/bin/sh
# Usage: eval-gate.sh BASELINE_JSON CMD [ARG...]
# CMD runs your eval and prints {"score": N} as its last line of stdout.
# BASELINE_JSON holds {"score": N, "commit": "SHA"}.
# Below baseline by more than EVAL_TOLERANCE (default 0.02): tells you, asks the
# agent to investigate, and exits 1. Otherwise reports the score and exits 0.
set -u
baseline_file=$1
shift
results_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tell-evals
mkdir -p "$results_dir" || exit 1
sha=$(git rev-parse --short HEAD) || exit 1
results=$results_dir/$sha-$(date +%Y%m%dT%H%M%S).txt

if ! "$@" >"$results"; then
    tell notify --type failure --title "Eval did not run" -- "$* failed at $sha. Output: $results" </dev/null
    exit 1
fi
score=$(tail -n 1 "$results" | jq -er '.score | numbers' 2>/dev/null) || {
    printf 'eval-gate: last line of %s has no numeric score\n' "$results" >&2
    exit 1
}
baseline=$(jq -er '.score | numbers' "$baseline_file") || exit 1
baseline_commit=$(jq -r '.commit // "unknown"' "$baseline_file")

if awk -v s="$score" -v b="$baseline" -v t="${EVAL_TOLERANCE:-0.02}" 'BEGIN { exit !(s < b - t) }'; then
    tell notify --type warning --title "Eval regression" \
        -- "Score $score vs baseline $baseline at $sha. Results: $results" </dev/null
    tell send --intent request -- "Eval score fell from $baseline (commit $baseline_commit) to $score at $sha.
Results: $results
Compare prompt, model, and retrieval changes in git log $baseline_commit..$sha and report the likely cause.
Do not edit $baseline_file." </dev/null
    exit 1
fi
tell notify --type success --title "Eval passed" -- "Score $score (baseline $baseline) at $sha" </dev/null
