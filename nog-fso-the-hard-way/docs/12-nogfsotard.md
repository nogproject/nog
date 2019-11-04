# Bootstrapping the Tar Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install a patched GNU Tar from <https://github.com/sprohaska/gnu-tar>, branch
`next^`, in order to use `tar --listed-incremental-mtime`:

```bash
apt-get install -y autoconf automake autopoint bison gcc make rsync texinfo wget

GNU_TAR_VERSION=5d82c6ca76c6afb9852c4cda6acf954a524c30ed && \
cd /tmp && \
git clone https://github.com/sprohaska/gnu-tar.git && \
cd gnu-tar && \
git checkout ${GNU_TAR_VERSION} && \
./bootstrap && \
FORCE_UNSAFE_CONFIGURE=1 ./configure prefix=/usr/local && \
make && \
make install && \
cd / && \
rm -rf /tmp/gnu-tar
```

Install `nogfsotard` and related programs:

```bash
apt-get install -y gnupg2 libcap2-bin sudo

mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2

for f in \
    git-receive-pack-get-owner \
    git-receive-pack-sudo-owner \
    nogfsoctl \
    nogfsoschd \
    nogfsotard \
    tartt \
    tartt-is-dir \
    tartt-store \
; do \
    install -m 0755 /tmp/fso/bin/${f} /usr/local/bin/${f} ; \
done

install -m 0755 -d /usr/local/lib/nogfsotard
for f in \
    git-archive-branch-dir \
    git-for-each-ref-dir \
    git-is-immutable-fso-stat-dir \
    git-is-newer-branch-dir-duration \
    git-rev-parse-branch-dir \
    git-rev-parse-is-valid-branch-dir \
    stat-dir-owner \
; do \
    install -m 0755 /tmp/fso/bin/${f} /usr/local/lib/nogfsotard/${f} ; \
done

rm -rf /tmp/fso
```

Configure Sudo to run helper programs:

```bash
install -m 0440 <<EOF /dev/stdin /etc/sudoers.d/nogfsotard
ngftar ALL=(root) NOPASSWD: /usr/local/bin/git-receive-pack-get-owner /*
ngftar ALL=(root) NOPASSWD: /usr/local/lib/nogfsotard/stat-dir-owner /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/bin/git-receive-pack
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-for-each-ref-dir /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-rev-parse-branch-dir master-stat /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-is-immutable-fso-stat-dir /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-is-newer-branch-dir-duration master-stat /* *
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-rev-parse-is-valid-branch-dir master-* /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/local/lib/nogfsotard/git-archive-branch-dir master-* /*
EOF
```

Configure helper programs with capabilities:

```bash
install -m 0750 -g ngftar /usr/local/bin/tar /usr/local/lib/nogfsotard/tar
setcap cap_dac_read_search=ep /usr/local/lib/nogfsotard/tar

install -m 0750 -g ngftar /usr/local/bin/tartt-is-dir /usr/local/lib/nogfsotard/tartt-is-dir
setcap cap_dac_read_search=ep /usr/local/lib/nogfsotard/tartt-is-dir
```

Configure `nogfsotard`:

Open <http://localhost:8080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsotard`:

```bash
NOG_JWT="eyJ..."

curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://nog.example.org:8080/api/v1/fso/sysauth \
| jq -r .data.token | tee /host/local/pki/jwt/nogfsotard.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsotard+storage",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "all" },
        { "action": "fso/read-registry", "names": ["exsrv", "exorg"] },
        {
            "actions": [
                "fso/read-repo",
                "fso/init-repo-tartt"
            ],
            "paths": [
                "/exsrv/*",
                "/exorg/*"
            ]
        }
    ]
}
EOF
```

Install etc files:

```bash
install -m 0755 -d /usr/local/etc/nogfsotard
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nogfsotard/fso-tls-cabundle.pem
install -m 0640 -g ngftar /host/local/pki/tls/nogfsotard-combined.pem /usr/local/etc/nogfsotard/fso-tls-combined.pem
install -m 0640 -g ngftar /host/local/pki/jwt/nogfsotard.jwt /usr/local/etc/nogfsotard/jwt

install -m 0644 <<EOF /dev/stdin /usr/local/etc/nogfsotard/.gitconfig
[user]
    name = tar daemon
    email = admin@example.org
EOF

ln -s /var/lib/nogfsotard/gnupg /usr/local/etc/nogfsotard/.gnupg
```

```bash
install -m 0644 <<\EOF /dev/stdin /usr/local/etc/nogfsotard/nogfsotardconfig.sh
# `cfgTarttUrlForRepo <repoId> <globalPath> <realdir>` prints the URL of the
# tartt repo used for the realdir archives of repo `<repoId>`.
#
# The function is only called if the URL is not yet stored in the registry.
cfgTarttUrlForRepo() {
    local id="$1"
    local globalPath="$2"

    local orgUnit
    case ${globalPath} in
    /exsrv/*/*)
        orgUnit="$(cut -d / -f 4 <<<"${globalPath}")"
        ;;
    /exorg/*)
        orgUnit="$(cut -d / -f 3 <<<"${globalPath}")"
        ;;
    *)
        echo >&2 "Unknown global path prefix."
        return 1
        ;;
    esac

    # Mangle the current year into the tar path.
    local year
    year="$(date +%Y)"

    local host='storage.example.org'
    local path="$(printf \
        '/srv/exorg_exsrv/tartt/%s/%s/%s/%s/%s.tartt' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    local tardir="$(printf \
        '/srv/samfs/tartt-tars/%s/%s/%s/%s/%s.tars' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    printf 'tartt://%s%s?driver=localtape&tardir=%s' \
        "${host}" "${path}" "${tardir}"
}

# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='storage.example.org'
# `cfgShadowRoots` lists valid shadow path prefixes, one per line.  Repos that
# use a different prefix are not processed.
cfgShadowRoots='
/srv/exorg_exsrv/shadow
'

# `cfgTarttStoreName` controls the Tartt store name.
cfgTarttStoreName='storage'

# `cfgTarEncryption` controls whether archives are encrypted.  Valid values:
#
#  - `none`: use `tartt tar --insecure-plaintext`
#  - `gpg`: use `tartt tar --recipient=...` or `tartt tar --plaintext-secret`
#
cfgTarEncryption='gpg'

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before starting an archive.  Archives are skipped
# if `df` reports less.
cfgCheckMinDf='
/srv/exorg_exsrv/tartt 10000
/srv/samfs/tartt-tars 10000
'

# `cfgBandwidthLimit` limits the data that is written per second.  It must be
# specified with a suffix `M` to indicate Megabytes per second.
cfgBandwidthLimit='60M'

# Set `cfgFakeArchives=t` to replace tar files with placeholders, which may
# be useful for testing.
cfgFakeArchives=

# To access shadow repos:
#
#  - use Sudo for Git read access;
#  - use capabilities for Tar read access.
#
# for detailed documentation.
cfgPrivGitRemote='sudo'
cfgPrivGitForEachRef='sudo'
cfgPrivGitArchive='sudo'
cfgCapPath='/usr/local/lib/nogfsotard'

# `cfgNogfsoschdTartt` defines the `nogfsoschd` command and arguments
# that control when to run tartt for realdirs.  Here:
#
#  - process repos with certain prefixes in certain registries;
#  - process a repo when `master-stat` changes;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdTartt() {
  nogfsoschd \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/usr/local/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsotard/fso-tls-cabundle.pem' \
      --sys-jwt='/usr/local/etc/nogfsotard/jwt' \
      --state='/var/lib/nogfsotard' \
      --scan-start \
      --scan-every='24h' \
      --host='storage.example.org' \
      --ref=refs/heads/master-stat \
      --registry='exsrv' \
      --registry='exorg' \
      --prefix='/exsrv' \
      --prefix='/exorg' \
      "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact with
# the registry, specifically to `nogfsoctl init-tartt`.
cfgNogfsoctl() {
  nogfsoctl \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/usr/local/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsotard/fso-tls-cabundle.pem' \
      --jwt-auth=no --jwt='/usr/local/etc/nogfsotard/jwt' \
      "$@"
}
EOF
```

Create the state and data directories:

```bash
install -m 0700 -o ngftar -d /var/lib/nogfsotard
install -m 0700 -o ngftar -d /var/lib/nogfsotard/gnupg
install -m 0750 -o ngftar -g ngftar -d /srv/exorg_exsrv/tartt
install -m 0750 -o ngftar -g ngftar -d /srv/samfs/tartt-tars
```

Start `nogfsotard`:

```bash
chroot --userspec=ngftar / \
env HOME='/usr/local/etc/nogfsotard' GIT_ALLOW_PROTOCOL=file:ext \
nogfsotard --config /usr/local/etc/nogfsotard/nogfsotardconfig.sh
```
