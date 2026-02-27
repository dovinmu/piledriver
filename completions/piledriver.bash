# Bash completion for piledriver
# Install: source this file or add to ~/.bashrc
# Or: cp piledriver.bash /etc/bash_completion.d/piledriver

_piledriver_completions() {
    local cur prev words cword
    _init_completion || return

    local commands="init set-phase bug test status check pr technique"
    local phases="RECONNAISSANCE SCOPING ASSUMPTIONS VERIFICATION REPORT"
    local techniques="tla+ property-test fuzz diff-test manual"

    # Complete commands
    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return
    fi

    local cmd="${words[1]}"

    case "$cmd" in
        init)
            # init takes session name (no completion) and optional --skip-recon
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--skip-recon" -- "$cur"))
            fi
            ;;
        set-phase)
            if [[ $cword -eq 2 ]]; then
                # Complete session names
                _piledriver_complete_sessions
            elif [[ $cword -eq 3 ]]; then
                # Complete phases
                COMPREPLY=($(compgen -W "$phases" -- "$cur"))
            elif [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--force" -- "$cur"))
            fi
            ;;
        bug)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            fi
            # bug name is free-form, no completion
            ;;
        test)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            elif [[ $cword -eq 3 ]]; then
                _piledriver_complete_bugs "${words[2]}"
            fi
            ;;
        status)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            fi
            ;;
        check)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            elif [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--sany" -- "$cur"))
            fi
            ;;
        pr)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            fi
            ;;
        technique)
            if [[ $cword -eq 2 ]]; then
                _piledriver_complete_sessions
            elif [[ $cword -eq 3 ]]; then
                COMPREPLY=($(compgen -W "$techniques" -- "$cur"))
            fi
            ;;
    esac
}

_piledriver_complete_sessions() {
    local piledriver_dir
    piledriver_dir=$(_piledriver_find_dir)
    if [[ -n "$piledriver_dir" && -d "$piledriver_dir" ]]; then
        local sessions
        sessions=$(find "$piledriver_dir" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | while read -r d; do basename "$d"; done | grep -v '^\.')
        COMPREPLY=($(compgen -W "$sessions" -- "$cur"))
    fi
}

_piledriver_complete_bugs() {
    local session="$1"
    local piledriver_dir
    piledriver_dir=$(_piledriver_find_dir)
    if [[ -n "$piledriver_dir" && -d "$piledriver_dir/$session/reproducers" ]]; then
        local bugs
        bugs=$(find "$piledriver_dir/$session/reproducers" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | while read -r d; do basename "$d"; done)
        COMPREPLY=($(compgen -W "$bugs" -- "$cur"))
    fi
}

_piledriver_find_dir() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
        if [[ -d "$dir/.piledriver" ]]; then
            echo "$dir/.piledriver"
            return
        fi
        dir=$(dirname "$dir")
    done
}

complete -F _piledriver_completions piledriver
