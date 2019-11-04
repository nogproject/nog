# Bootstrapping the Shadow Backup Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Create the data directory:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m 0750 -o ngfbak -g ngfbak -d /srv/samfs/nogfsobak
EOF
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsosdwbakd3`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/pki/jwt/nogfsosdwbakd3.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsosdwbakd3+storage",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "all" },
        { "action": "fso/read-registry", "names": ["exsrv", "exorg"] },
        {
            "actions": [
                "fso/read-repo",
                "fso/init-repo-shadow-backup"
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
    install -m 0755 </dev/null -d /etc/nogfsosdwbakd3

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/tls/ca.pem /dev/stdin /etc/nogfsosdwbakd3/fso-tls-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfbak <local/pki/tls/nogfsosdwbakd3-combined.pem /dev/stdin /etc/nogfsosdwbakd3/fso-tls-combined.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfbak <local/pki/jwt/nogfsosdwbakd3.jwt /dev/stdin /etc/nogfsosdwbakd3/nogfsosdwbakd3.jwt
```

Configure `nogfsosdwbakd3`:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/nogfsosdwbakd3/nogfsosdwbakd3config.sh
# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='storage.example.org'
# `cfgShadowRoots` lists valid shadow repository path prefixes, one per line.
# Backup Tar archives will contain paths relative to one of the shadow roots.
cfgShadowRoots='
/srv/exorg_exsrv/shadow
'

# `cfgShadowBackupUrlForRepo <repoId> <globalPath>` prints the URL of the
# backups of the shadow repo `<repoId>`.
#
# The function is only called if the backup path is not yet stored in the
# registry.
cfgShadowBackupUrlForRepo() {
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

    # Mangle the current year into the backup path.
    local year
    year="$(date +%Y)"

    printf \
        'nogfsobak://storage.example.org/srv/samfs/nogfsobak/%s/%s/%s/%s/%s' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}"
}

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before a backup.  Backups are skipped if `df`
# reports less.
cfgCheckMinDf='
/srv/samfs/nogfsobak 10000
'

# `cfgBuckets` is a list of `<bucket> <max> <selector>...`.  The latest
# backup is added to `<bucket>` if `find -type f <selector>` does not match
# the most recent file in the bucket.  The oldest backups are deleted if a
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

# `cfgCapPath` contains programs with capabilities.  See `Dockerfile.jnj`.
cfgCapPath='/usr/lib/nogfsosdwbakd3'

# `cfgPrivGitForEachRef` controls how to run `git for-each-ref`.
#
#  - `none`: Use `git`.
#  - `sudo`: Use sudo to determine the owner of the repo and then use sudo to
#    run `git for-each-ref` as that user.
#  - `cap`: Use `git` from `cfgCapPath`.
#
cfgPrivGitForEachRef='sudo'

# `cfgNogfsoschdShadowBackup` defines the `nogfsoschd` command and arguments
# that control when to back up a shadow repo.  Here:
#
#  - process repos for Nogfsostad registries and prefixes;
#  - process a repo on any ref update;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdShadowBackup() {
  nogfsoschd \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/etc/nogfsosdwbakd3/fso-tls-combined.pem' \
      --tls-ca='/etc/nogfsosdwbakd3/fso-tls-cabundle.pem' \
      --sys-jwt='/etc/nogfsosdwbakd3/nogfsosdwbakd3.jwt' \
      --state='/var/lib/nogfsosdwbakd3' \
      --scan-start \
      --scan-every='1h' \
      --host='storage.example.org' \
      --registry='exsrv' \
      --registry='exorg' \
      --prefix='/exsrv' \
      --prefix='/exorg' \
      "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact
# with the registry, specifically to `nogfsoctl init-shadow-backup`.
cfgNogfsoctl() {
  nogfsoctl \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/etc/nogfsosdwbakd3/fso-tls-combined.pem' \
      --tls-ca='/etc/nogfsosdwbakd3/fso-tls-cabundle.pem' \
      --jwt-auth=no --jwt='/etc/nogfsosdwbakd3/nogfsosdwbakd3.jwt' \
      "$@"
}
EOF
```

Allow `nogfsosdwbakd3` to `sudo`:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
sed -i -e '/^#ngf/ s/^#//' /etc/sudoers.d/nogfsosdwbakd3
sed -i -e '/^# Uncomment/d' /etc/sudoers.d/nogfsosdwbakd3
EOF
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsosdwbakd3.service
[Unit]
Description=nogfsosdwbakd3

[Service]
Restart=always
User=ngfbak
Environment='HOME=/var/lib/nogfsosdwbakd3'
ExecStart=\
    '/usr/bin/nogfsosdwbakd3' \
        '--config' \
        '/etc/nogfsosdwbakd3/nogfsosdwbakd3config.sh' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/samfs/nogfsobak
# Writable `/var/lib/nogfsosdwbakd3`.
StateDirectory=nogfsosdwbakd3
# `sudo` requires a writable `/run`.
TemporaryFileSystem=/run
PrivateTmp=yes
ProtectHome=yes
ProtectControlGroups=yes
# Allow `sudo`.
CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE
# Allow `/usr/lib/nogfsosdwbakd3/tar`.
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

Start the `nogfsosdwbakd3` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsosdwbakd3
systemctl start nogfsosdwbakd3
systemctl status nogfsosdwbakd3
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsosdwbakd3 -f
```
