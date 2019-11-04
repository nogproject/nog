# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='files.example.com'
# `cfgShadowRoots` lists valid shadow repository path prefixes, one per line.
# Backup tarballs will contain paths relative to one of the shadow roots.
cfgShadowRoots='
/nogfso/legacy-shadow
/nogfso/shadow
'

# `cfgShadowBackupUrlForRepo <repoId> <globalPath>` prints the URL of the
# backups of the shadow repo `<repoId>`.
#
# The function is only called if the backup path is not yet stored in the
# registry.
cfgShadowBackupUrlForRepo() {
    local id="$1"
    local globalPath="$2"

    # Mangle the current time into the path to simulate a time-based naming
    # scheme, e.g. a scheme that organizes backup directories by creation year.
    local ts
    ts="$(date +%s)"

    local orgUnit
    case ${globalPath} in
    /example/orgfs2/srv/*/*)
        orgUnit="$(cut -d / -f 6 <<<"${globalPath}")"
        ;;
    /example/orgfs2/org/*)
        orgUnit="$(cut -d / -f 5 <<<"${globalPath}")"
        ;;
    *)
        echo >&2 "Unknown global path prefix."
        return 1
        ;;
    esac

    local host='files.example.com'
    local path="$(printf \
        '/nogfso/backup/shadow/%s/%s/%s/%s/%s' \
        "${ts}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    printf 'nogfsobak://%s%s' "${host}" "${path}"
}

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before a backup.  Backups will be skipped if `df`
# reports less.
cfgCheckMinDf='
/nogfso/backup 1000000
'

# `cfgBuckets` is a list of `<bucket> <max> <selector>...`.  The latest backup
# will be added to `<bucket>` if `find -type f <selector>` does not match the
# most recent file in the bucket.  The oldest backups will be deleted if a
# bucket contains more than `<max>` files.
#
# To ensure that the latest state is always in at least one bucket, bucket
# `latest` uses `-false`, so that it receives every backup.
cfgBuckets='
latest 2 -false
hourly 10 -mmin -60
daily 7 -mmin -1440
weekly 5 -mtime -7
monthly 4 -mtime -30
'

# `cfgCapPath` is a directory that contains binaries that are used when reading
# from the real and shadow dirs.  It usually contains the following binaries
# with capabilities:
#
# ```
# setcap cap_dac_read_search=ep git
# setcap cap_dac_read_search=ep tar
# ```
#
cfgCapPath='/usr/local/lib/nogfsosdwbakd3'

# `cfgPrivGitForEachRef` controls how to run `git for-each-ref`.
#
#  - `none`: Use `git`.
#  - `cap`: Use `git` from `cfgCapPath`.
#  - `sudo`: Use sudo to determine the repo owner, and then use sudo to run
#    `git for-each-ref` as the repo owner.  See details below.
#
# `sudo` expects the following wrapper scripts:
#
# ```
# /usr/local/lib/nogfsosdwbakd3/git-for-each-ref-dir
# ```
#
# and requires sudoer rights:
#
# ```
# ngfbak ALL=(root) NOPASSWD: /usr/bin/stat --format=%U -- /*
# ngfbak ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsosdwbakd3/git-for-each-ref-dir /*
# ```
#
# Where `ngfbak` is the user that runs `nogfsosdwbakd3` and
# `ngfsta2,ngfsta3,...` are the users that own the shadow repos.
cfgPrivGitForEachRef='sudo'

# `cfgNogfsoschdShadowBackup` defines the `nogfsoschd` command and arguments
# that control when to back up a shadow repo.  Here:
#
#  - process repos with prefix `/example/orgfs2` in registry `exreg`;
#  - process a repo on any ref updates;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdShadowBackup() {
    nogfsoschd \
        --log=mu \
        --tls-cert=/nog/ssl/certs/nogfsosdwbakd3/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsosdwbakd3/ca.pem \
        --sys-jwt="/nog/jwt/tokens/nogfsosdwbakd3.jwt" \
        --state='/nogfso/var/lib/nogfsosdwbakd3' \
        --host="${cfgShadowHost}" \
        --registry=exreg \
        --prefix=/example/orgfs2 \
        --scan-start \
        --scan-every=1h \
        "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact with
# the registry, specifically to `nogfsoctl init-shadow-backup`.
cfgNogfsoctl() {
    nogfsoctl \
        --tls-cert=/nog/ssl/certs/nogfsosdwbakd3/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsosdwbakd3/ca.pem \
        --jwt-auth=no --jwt="/nog/jwt/tokens/nogfsosdwbakd3.jwt" \
        "$@"
}
