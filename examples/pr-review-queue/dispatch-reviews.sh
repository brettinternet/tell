#!/bin/sh
# Ask the reviewer agent to review each newly requested PR, once.
# Run on a schedule that never overlaps runs (Noon overlap: skip, launchd,
# systemd timers). Remembers dispatched PR URLs; Tell keeps no state.
set -u
state_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tell-pr-reviews
seen=$state_dir/seen
mkdir -p "$state_dir" && touch "$seen" || exit 1

urls=$(gh search prs --review-requested=@me --state=open --json url --jq '.[].url') || exit 1

printf '%s\n' "$urls" | while read -r url; do
    [ -n "$url" ] || continue
    grep -Fxq "$url" "$seen" && continue

    # The URL only: titles and descriptions are written by others and must not
    # become instructions. The agent reads them with gh, as data.
    tell send --intent request -- "Review $url (read it with gh pr view/diff).
Report blockers and questions to me. Do not approve, merge, push, or comment on GitHub." </dev/null
    case $? in
        0) printf '%s\n' "$url" >>"$seen" ;;
        3) ;; # nothing sent (agent absent or busy): the next run tries again
        *)
            # Failed or uncertain: it may have arrived. Record it and ask a person.
            printf '%s\n' "$url" >>"$seen"
            tell notify --type warning --title "Review dispatch unclear" \
                -- "$url may not have reached the reviewer agent. Check before resending." </dev/null || true
            ;;
    esac
done
