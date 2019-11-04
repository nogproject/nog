# `cfgTarttUrlForRepo <repoId> <globalPath> <realdir>` prints the URL of the
# tartt repo used for the realdir archives of repo `<repoId>`.
#
# The function is only called if the URL is not yet stored in the registry.
cfgTarttUrlForRepo() {
    local id="$1"
    local globalPath="$2"
    local realdir="$3"

    # Mangle the current time into the archive path to simulate a time-based
    # naming scheme, e.g. a scheme that organizes tartt repos by creation year.
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
        '/nogfso/archive/tartt/%s/%s/%s/%s/%s.tartt' \
        "${ts}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    local tardir="$(printf \
        '/nogfso/tape/tartt/%s/%s/%s/%s/%s.tars' \
        "${ts}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    printf 'tartt://%s%s?driver=localtape&tardir=%s' \
        "${host}" "${path}" "${tardir}"
}

# `cfgTarEncryption` controls whether archives are encrypted.  Valid values:
#
#  - `none`: `tartt tar --insecure-plaintext`
#  - `gpg`: `tartt tar --recipient=...` or `tartt tar --plaintext-secret`,
#    depending on per-repo configuration.
#
# It can be useful to change it temporarily to `none` during testing.
cfgTarEncryption='gpg'

# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='files.example.com'
# `cfgShadowRoots` lists valid shadow path prefixes, one per line.  Repos that
# use a different prefix are not processed.
cfgShadowRoots='
/nogfso/legacy-shadow
/nogfso/shadow
'

# `cfgTarttStoreName` is an optional variable that, if set, is used as the
# store name for new repos with `tartt init store=${cfgTarttStoreName} ...`.
cfgTarttStoreName='files'

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before starting an archive.  Archives are skipped
# if `df` reports less.
cfgCheckMinDf='
/nogfso/archive 1000000
/nogfso/tape 1000000
'

# `cfgBandwidthLimit` limits the data that is written per second.  It must be
# specified with a suffix `M` to indicate Megabytes per second.
cfgBandwidthLimit='50M'

# Set `cfgFakeArchives=t` to replace tar files with placeholders, which may
# be useful for testing.
cfgFakeArchives=t

# `cfgNogfsoschdTartt` defines the `nogfsoschd` command and arguments
# that control when to run tartt for realdirs.  Here:
#
#  - process repos with prefix `/example/orgfs2` in registry `exreg`;
#  - process a repo when `master-stat` changes;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdTartt() {
    nogfsoschd \
        --log=mu \
        --tls-cert=/nog/ssl/certs/nogfsotard/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsotard/ca.pem \
        --sys-jwt="/nog/jwt/tokens/nogfsotard.jwt" \
        --state='/nogfso/var/lib/nogfsotard' \
        --host="${cfgShadowHost}" \
        --ref=refs/heads/master-stat \
        --registry=exreg \
        --prefix=/example/orgfs2 \
        --scan-start \
        --scan-every=1h \
        "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact with
# the registry, specifically to `nogfsoctl init-tartt`.
cfgNogfsoctl() {
    nogfsoctl \
        --tls-cert=/nog/ssl/certs/nogfsotard/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsotard/ca.pem \
        --jwt-auth=no --jwt="/nog/jwt/tokens/nogfsotard.jwt" \
        "$@"
}

# `cfgPrivGitRemote` controls how to push the branch `master-tartt` to the
# shadow repo:
#
#  - `none`: Use a normal Git remote.
#  - `sudo`: Use sudo to determine the repo owner, and then use use sudo with
#    `git-remote-ext` to run `git-receive-pack` as the repo owner.  See details
#    below.
#
# `sudo` requires sudoers rights:
#
# ```
# ngftar ALL=(root) NOPASSWD: /usr/bin/stat --format=%U -- /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/bin/git-receive-pack
# ```
#
# Where `ngftar` is the user that runs `nogfsotard` and `ngfsta2,ngfsta3,...`
# are the users that own the shadow repos.
cfgPrivGitRemote='sudo'

# `cfgPrivGitForEachRef` controls how to read Git refs from a shadow repo:
#
#  - `none`: Use `git`.
#  - `cap`: Use `git` from `cfgCapPath`.
#  - `sudo`: Use sudo to determine the repo owner, and then use sudo with Git
#    wrapper scripts to read from the repo.  See details below.
#
# `sudo` expects the following wrapper scripts:
#
# ```
# /usr/local/lib/nogfsotard/git-for-each-ref-dir
# /usr/local/lib/nogfsotard/git-rev-parse-branch-dir
# /usr/local/lib/nogfsotard/git-is-newer-branch-dir-duration
# /usr/local/lib/nogfsotard/git-is-immutable-fso-stat-dir
# ```
#
# and requires sudoer rights:
#
# ```
# ngftar ALL=(root) NOPASSWD: /usr/bin/stat --format=%U -- /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-for-each-ref-dir /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-rev-parse-branch-dir master-stat /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-is-newer-branch-dir-duration master-stat /* *
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-is-immutable-fso-stat-dir /*
# ```
#
# Where `ngftar` is the user that runs `nogfsotard` and `ngfsta2,ngfsta3,...`
# are the users that own the shadow repos.
cfgPrivGitForEachRef='sudo'

# `cfgPrivGitArchive` controls how to tar Git refs from a shadow repo:
#
#  - `none`: Use `git`.
#  - `cap`: Use `git` from `cfgCapPath`.
#  - `sudo`: Use sudo to determine the repo owner, and then use sudo with Git
#    wrapper scripts to read from the repo.  See details below.
#
# `sudo` expects the following wrapper scripts:
#
# ```
# /usr/local/lib/nogfsotard/git-rev-parse-is-valid-branch-dir
# /usr/local/lib/nogfsotard/git-archive-branch-dir
# ```
#
# and requires sudoer rights:
#
# ```
# ngftar ALL=(root) NOPASSWD: /usr/bin/stat --format=%U -- /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-rev-parse-is-valid-branch-dir master-* /*
# ngftar ALL=(ngfsta2,ngfsta3) NOPASSWD: /usr/local/lib/nogfsotard/git-archive-branch-dir master-* /*
# ```
#
# Where `ngftar` is the user that runs `nogfsotard` and `ngfsta2,ngfsta3,...`
# are the users that own the shadow repos.
cfgPrivGitArchive='sudo'

# `cfgCapPath` is a directory that contains binaries that are used when reading
# from the real and shadow dirs.  It usually contain the following binaries
# with capabilities:
#
# ```
# setcap cap_dac_read_search=ep git
# setcap cap_dac_read_search=ep tar
# ```
#
cfgCapPath='/usr/local/lib/nogfsotard'
