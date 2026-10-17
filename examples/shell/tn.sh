# shellcheck shell=bash
# Source from ~/.zshrc or ~/.bashrc:  . /path/to/tn.sh
# tn CMD [ARG...]  runs CMD, then notifies with its exit status and duration.
# The command's own exit status is preserved; notification failures are ignored.
tn() {
    local start status elapsed
    start=$(date +%s)
    "$@"
    status=$?
    elapsed=$(($(date +%s) - start))
    if [ "$status" -eq 0 ]; then
        tell notify --type success --title "Done: $1" -- "$* finished in ${elapsed}s" </dev/null
    else
        tell notify --type failure --title "Failed: $1" -- "$* exited $status after ${elapsed}s" </dev/null
    fi
    return "$status"
}
