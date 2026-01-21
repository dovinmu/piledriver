# Fish completion for piledriver
# Install: copy to ~/.config/fish/completions/piledriver.fish

set -l commands init set-phase bug test status check pr technique

# Disable file completion by default
complete -c piledriver -f

# Commands
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a init -d "Start a new analysis session"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a set-phase -d "Transition to a new phase"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a bug -d "Create a bug reproducer scaffold"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a test -d "Run before/after test validation"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a status -d "Show current state"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a check -d "Run TLC model checker"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a pr -d "Generate PR draft from report"
complete -c piledriver -n "not __fish_seen_subcommand_from $commands" -a technique -d "View or set verification technique"

# Helper function to find .piledriver directory
function __piledriver_find_dir
    set -l dir (pwd)
    while test "$dir" != "/"
        if test -d "$dir/.piledriver"
            echo "$dir/.piledriver"
            return
        end
        set dir (dirname "$dir")
    end
end

# Helper function to list sessions
function __piledriver_sessions
    set -l pd_dir (__piledriver_find_dir)
    if test -n "$pd_dir" -a -d "$pd_dir"
        for d in $pd_dir/*/
            basename $d
        end
    end
end

# Helper function to list bugs for a session
function __piledriver_bugs
    set -l pd_dir (__piledriver_find_dir)
    set -l session $argv[1]
    if test -n "$pd_dir" -a -d "$pd_dir/$session/reproducers"
        for d in $pd_dir/$session/reproducers/*/
            basename $d
        end
    end
end

# init: session name + optional --skip-recon
complete -c piledriver -n "__fish_seen_subcommand_from init" -l skip-recon -d "Skip RECONNAISSANCE, start in SCOPING"

# set-phase: session name, then phase
complete -c piledriver -n "__fish_seen_subcommand_from set-phase; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"
complete -c piledriver -n "__fish_seen_subcommand_from set-phase; and test (count (commandline -opc)) -eq 3" -a "RECONNAISSANCE SCOPING ASSUMPTIONS VERIFICATION REPORT" -d "Phase"
complete -c piledriver -n "__fish_seen_subcommand_from set-phase" -l force -d "Force transition even if validation fails"

# bug: session name, then bug name
complete -c piledriver -n "__fish_seen_subcommand_from bug; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"

# test: session name, then optional bug name
complete -c piledriver -n "__fish_seen_subcommand_from test; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"
complete -c piledriver -n "__fish_seen_subcommand_from test; and test (count (commandline -opc)) -eq 3" -a "(__piledriver_bugs (commandline -opc)[3])" -d "Bug"

# status: optional session name
complete -c piledriver -n "__fish_seen_subcommand_from status; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"

# check: session name + optional --sany
complete -c piledriver -n "__fish_seen_subcommand_from check; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"
complete -c piledriver -n "__fish_seen_subcommand_from check" -l sany -d "Syntax check only (no model checking)"

# pr: session name
complete -c piledriver -n "__fish_seen_subcommand_from pr; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"

# technique: session name, then optional technique
complete -c piledriver -n "__fish_seen_subcommand_from technique; and test (count (commandline -opc)) -eq 2" -a "(__piledriver_sessions)" -d "Session"
complete -c piledriver -n "__fish_seen_subcommand_from technique; and test (count (commandline -opc)) -eq 3" -a "tla+ property-test fuzz diff-test manual" -d "Technique"
