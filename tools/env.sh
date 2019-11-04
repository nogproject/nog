# Source the file in Bash.

alias dc='docker-compose'
alias dev='docker-compose run dev'
alias ddev='echo DEPRECATED: use godev instead && docker-compose run godev'
alias godev='docker-compose run godev'

# `nogfsoctl` without TTY, so that `vid=$(nogfsoctl ...)` works as expected.
alias nogfsoctl='docker-compose run -T godev nogfsoctl'

if [ -f '.git' ]; then
    cat <<\EOF
**************************************************************************
Warning: `.git` is a file.  Move the git dir, so that Git commands work in
Docker containers:

    cat .git \
    && git submodule foreach --recursive 'git config --unset core.worktree || :' \
    && git submodule deinit --all \
    && gitdir="$(git rev-parse --git-dir)" \
    && git config --unset core.worktree \
    && rm .git \
    && mv "${gitdir}" .git \
    && git submodule update --init \
    && git status

EOF
fi
