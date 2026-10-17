# Source from ~/.zshrc. Notifies when an interactive command runs at least
# TELL_LONG_SECONDS (default 30), so you can look away from slow builds.
# Needs a visible route (desktop, herdr, command); output is discarded so a
# background delivery cannot scribble over your prompt.
zmodload zsh/datetime
autoload -Uz add-zsh-hook

typeset -g _tell_cmd= _tell_start=

_tell_preexec() {
    _tell_cmd=$1
    _tell_start=$EPOCHSECONDS
}

_tell_precmd() {
    local exit_status=$?
    [[ -n $_tell_start ]] || return 0
    local elapsed=$((EPOCHSECONDS - _tell_start)) cmd=$_tell_cmd
    _tell_start= _tell_cmd=
    ((elapsed >= ${TELL_LONG_SECONDS:-30})) || return 0
    # Interactive programs are long because you were using them.
    case ${cmd%% *} in
        vi | vim | nvim | less | man | ssh | top | htop | btop | tmux | herdr | lazygit | claude | codex | pi) return 0 ;;
    esac
    local type=success
    ((exit_status == 0)) || type=failure
    tell --timeout 5s notify --type $type --title "${cmd[1,60]}" \
        -- "Exited $exit_status after ${elapsed}s" </dev/null >/dev/null 2>&1 &!
}

add-zsh-hook preexec _tell_preexec
add-zsh-hook precmd _tell_precmd
