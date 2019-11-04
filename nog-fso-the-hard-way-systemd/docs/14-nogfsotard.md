# Bootstrapping the Tar Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Create data directories:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m 0750 -o ngftar -g ngftar -d /srv/exorg_exsrv/tartt
install -m 0750 -o ngftar -g ngftar -d /srv/samfs/tartt-tars
EOF
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
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
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/pki/jwt/nogfsotard.jwt
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

Install certificates and JWT:

```bash
vagrant ssh storage -- sudo \
    install -m 0755 </dev/null -d /etc/nogfsotard

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/tls/ca.pem /dev/stdin /etc/nogfsotard/fso-tls-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngftar <local/pki/tls/nogfsotard-combined.pem /dev/stdin /etc/nogfsotard/fso-tls-combined.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngftar <local/pki/jwt/nogfsotard.jwt /dev/stdin /etc/nogfsotard/nogfsotard.jwt
```

Install a Git configuration:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/nogfsotard/gitconfig
[user]
    name = tar daemon
    email = admin@example.org
EOF
```

Configure `nogfsotard`:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/nogfsotard/nogfsotardconfig.sh
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
cfgCapPath='/usr/lib/nogfsotard'

# `cfgNogfsoschdTartt` defines the `nogfsoschd` command and arguments
# that control when to run tartt for realdirs.  Here:
#
#  - process repos with certain prefixes in certain registries;
#  - process a repo when `master-stat` changes;
#  - scan at start and regular intervals as a fallback if ref updates get lost.
#
cfgNogfsoschdTartt() {
  nogfsoschd \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/etc/nogfsotard/fso-tls-cabundle.pem' \
      --sys-jwt='/etc/nogfsotard/nogfsotard.jwt' \
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
      --tls-cert='/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/etc/nogfsotard/fso-tls-cabundle.pem' \
      --jwt-auth=no --jwt='/etc/nogfsotard/nogfsotard.jwt' \
      "$@"
}
EOF
```

Allow `nogfsotard` to `sudo`:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
sed -i -e '/^#ngf/ s/^#//' /etc/sudoers.d/nogfsotard
sed -i -e '/^# Uncomment/d' /etc/sudoers.d/nogfsotard
EOF
```

Create the state directory:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m 0755 -o ngftar -g ngftar -d /var/lib/nogfsotard
install -m 0700 -o ngftar -g ngftar -d /var/lib/nogfsotard/.gnupg
ln -snf /etc/nogfsotard/gitconfig /var/lib/nogfsotard/.gitconfig
EOF
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsotard.service
[Unit]
Description=nogfsotard

[Service]
Restart=always
User=ngftar
Environment='HOME=/var/lib/nogfsotard'
Environment='GIT_ALLOW_PROTOCOL=file:ext'
ExecStart=\
    '/usr/bin/nogfsotard' \
        '--config' \
        '/etc/nogfsotard/nogfsotardconfig.sh' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/exorg_exsrv/shadow
ReadWritePaths=/srv/exorg_exsrv/tartt
ReadWritePaths=/srv/samfs/tartt-tars
ReadWritePaths=/var/lib/nogfsotard
# `sudo` requires a writable `/run`.
TemporaryFileSystem=/run
PrivateTmp=yes
ProtectHome=yes
ProtectControlGroups=yes
# Allow `sudo`.
CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE
# Allow `/usr/lib/nogfsotard/tar`.
CapabilityBoundingSet=CAP_DAC_READ_SEARCH

# Do not use the following protections, because `sudo` would fail.
NoNewPrivileges=no
ProtectKernelTunables=no
ProtectKernelModules=no
PrivateDevices=no
SystemCallArchitectures=

[Install]
WantedBy=default.target
EOF
```

Start the `nogfsotard` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsotard
systemctl start nogfsotard
systemctl status nogfsotard
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsotard -f
```
