# Bootstrapping the Main File Server Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Initialize the shadow toplevel directories:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m u=rwx,g=rxs,o= -g ngfsta -d /srv/exorg_exsrv/shadow

find /srv/exorg_exsrv/data -mindepth 1 -maxdepth 2 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=rxs,o=- -g ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=s,o=- -o ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '%P\0' \
| xargs -0 --verbose -n 1 -i chgrp '--reference=/srv/exorg_exsrv/data/{}' '/srv/exorg_exsrv/shadow/{}'

find /srv/exorg_exsrv/shadow/ -ls
EOF
```

Initialize the spool directories:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m 0755 -d /srv/exorg_exsrv/data/.spool
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/archive-repo
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/unarchive-repo

find /srv/exorg_exsrv/data/.spool/ -ls
EOF
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsostad`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/pki/jwt/nogfsostad.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsostad+storage",
    "san": ["DNS:storage.example.org"],
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/write", "name": "all" },
        { "action": "bc/read", "name": "allaggsig" },
        { "action": "fso/session", "name": "storage.example.org" },
        {
            "actions": [
                "fso/read-registry",
                "fso/exec-ping-registry"
            ],
            "names": ["exsrv", "exorg"]
        },
        {
            "actions": [
                "fso/read-root",
                "fso/exec-du",
                "fso/exec-split-root",
                "fso/read-repo",
                "fso/confirm-repo",
                "fso/exec-freeze-repo",
                "fso/exec-unfreeze-repo",
                "fso/exec-archive-repo",
                "fso/exec-unarchive-repo"
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
    install -m 0755 </dev/null -d /etc/nogfsostad

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/tls/ca.pem /dev/stdin /etc/nogfsostad/fso-tls-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfsta <local/pki/tls/nogfsostad-combined.pem /dev/stdin /etc/nogfsostad/fso-tls-combined.pem

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/jwt/ca.pem /dev/stdin /etc/nogfsostad/fso-jwt-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfsta <local/pki/jwt/nogfsostad.jwt /dev/stdin /etc/nogfsostad/nogfsostad.jwt
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsostad.service
[Unit]
Description=nogfsostad

[Service]
Restart=always
Environment='HOME=/'
ExecStart=\
    '/usr/bin/nogfsostasvsd' \
        '--userspec=ngfsta' \
        '--group-prefix=ag_exorg' \
        '--group-prefix=exsrv_' \
        '--group-prefix=exorg_' \
        '--' \
    '/usr/bin/nogfsostad' \
        '--nogfsoregd=fso.example.org:7550' \
        '--tls-cert=/etc/nogfsostad/fso-tls-combined.pem' \
        '--tls-ca=/etc/nogfsostad/fso-tls-cabundle.pem' \
        '--jwt-ca=/etc/nogfsostad/fso-jwt-cabundle.pem' \
        '--jwt-ou=nog-jwt' \
        '--jwt-unix-domain=EXO' \
        '--sys-jwt=/etc/nogfsostad/nogfsostad.jwt' \
        '--session-name=storage.example.org' \
        '--git-fso-program=/usr/bin/git-fso' \
        '--sududod-socket=/run/nogfsostad/sududod/sock' \
        '--shutdown-timeout=40s' \
        '--log=prod' \
        '--gitlab=no' \
        '--git-committer=nogfsostad <admin@example.org>' \
        '--trim-host-root=/srv/exorg_exsrv/data' \
        '--shadow-root=/srv/exorg_exsrv/shadow' \
        '--archive-repo-spool=/srv/exorg_exsrv/data/.spool/archive-repo' \
        '--unarchive-repo-spool=/srv/exorg_exsrv/data/.spool/unarchive-repo' \
        '--git-gc-scan-start=20m' \
        '--git-gc-scan-every=240h' \
        '--stat-author=nogfsostad <admin@example.org>' \
        '--stat-scan-start=10m' \
        '--stat-scan-every=24h' \
        '--init-limit-max-files=2k' \
        '--init-limit-max-size=20g' \
        '--prefix-init-limit=/srv/exorg_exsrv/data/exsrv/tem-707:2k:250g' \
        '--host=storage.example.org' \
        '--prefix=/exsrv' \
        '--prefix=/exorg' \
        'exsrv' \
        'exorg' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/exorg_exsrv/data/.spool
ReadWritePaths=/srv/exorg_exsrv/shadow
ProtectHome=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes
ProtectKernelModules=yes
PrivateDevices=yes
SystemCallArchitectures=native
NoNewPrivileges=yes
# Allow nogfsostasvsd to use chroot to switch user and groups.
CapabilityBoundingSet=CAP_SYS_CHROOT CAP_SETUID CAP_SETGID

[Install]
WantedBy=default.target
EOF
```

Start the `nogfsostad` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsostad
systemctl start nogfsostad
systemctl status nogfsostad
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsostad -f
```
