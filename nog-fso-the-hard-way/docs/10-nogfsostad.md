# Bootstrapping the Main File Server Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install `nogfsostad` and related programs:

```bash
apt-get install -y bc gawk git jq

rm /usr/share/git-core/templates/hooks/*.sample

mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2
install -m 0755 /tmp/fso/bin/nogfsostad /usr/local/bin/nogfsostad
install -m 0755 /tmp/fso/bin/git-fso /usr/local/bin/git-fso
rm -rf /tmp/fso
```

Open <http://localhost:8080> in Chrome, and issue a temporary JWT by executing
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
    http://nog.example.org:8080/api/v1/fso/sysauth \
| jq -r .data.token | tee /host/local/pki/jwt/nogfsostad.jwt
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

Add server groups and users:

* We add more users and groups than strictly needed at this point to run
  `nogfsostad`.  The additional users and groups will be used to run additional
  daemons with privilege separation.  A single daemon user that would be shared
  by all daemons would in principle be sufficient.

```bash
srvGroups='
ngfsta
ngftar
ngfbak
'

# Lines: <user> <primary-group> <secondary-groups>...
srvUsers='
ngfsta ngfsta ag_exorg exorg_ag-alice exorg_ag-bob exorg_ag-charly exorg_em-facility exorg_lm-facility exorg_ms-facility exsrv_spim-100 exsrv_spim-222 exsrv_tem-505 exsrv_rem-707 exsrv_ms-data
ngftar ngftar
ngfrst ngfsta ngftar
ngfbak ngfbak
'

for group in ${srvGroups}; do
    addgroup "${group}"
done

grep -v '^ *$' <<<"${srvUsers}" | while read -r user grp1 grps2; do
    adduser --system --shell /bin/bash --ingroup "${grp1}" "${user}"
    if [ -z "${grps2}" ]; then
        continue
    fi
    for g in ${grps2}; do
        usermod -aG "${g}" "${user}"
        echo "Added user \`${user}\` to group \`${g}\`."
    done
done
```

Install certs:

```bash
install -m 0755 -d /usr/local/etc/nogfsostad
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nogfsostad/fso-tls-cabundle.pem
install -m 0640 -g ngfsta /host/local/pki/tls/nogfsostad-combined.pem /usr/local/etc/nogfsostad/fso-tls-combined.pem
install -m 0644 /host/local/pki/jwt/ca.pem /usr/local/etc/nogfsostad/fso-jwt-cabundle.pem
install -m 0640 -g ngfsta /host/local/pki/jwt/nogfsostad.jwt /usr/local/etc/nogfsostad/jwt
```

Initialize the shadow toplevel directories:

```bash
install -m u=rwx,g=rxs,o= -g ngfsta -d /srv/exorg_exsrv/shadow

find /srv/exorg_exsrv/data -mindepth 1 -maxdepth 2 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=rxs,o=- -g ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=s,o=- -o ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '%P\0' \
| xargs -0 --verbose -n 1 -i chgrp '--reference=/srv/exorg_exsrv/data/{}' '/srv/exorg_exsrv/shadow/{}'

find /srv/exorg_exsrv/shadow/ -ls
```

Initialize the spool directories:

```bash
install -m 0755 -d /srv/exorg_exsrv/data/.spool
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/archive-repo
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/unarchive-repo

find /srv/exorg_exsrv/data/.spool/ -ls
```

Start `nogfsostad`:

```bash
chroot --userspec=ngfsta / \
env HOME=/ \
nogfsostad \
    --nogfsoregd=fso.example.org:7550 \
    --tls-cert=/usr/local/etc/nogfsostad/fso-tls-combined.pem \
    --tls-ca=/usr/local/etc/nogfsostad/fso-tls-cabundle.pem \
    --jwt-ca=/usr/local/etc/nogfsostad/fso-jwt-cabundle.pem \
    --jwt-ou=nog-jwt \
    --jwt-unix-domain=EXO \
    --sys-jwt=/usr/local/etc/nogfsostad/jwt \
    --session-name=storage.example.org \
    --git-fso-program=/usr/local/bin/git-fso \
    --sududod-socket=/run/nogfsostad/sududod/sock \
    --shutdown-timeout=40s \
    --log=prod \
    --gitlab=no \
    --git-committer='nogfsostad <admin@example.org>' \
    --trim-host-root='/srv/exorg_exsrv/data' \
    --shadow-root='/srv/exorg_exsrv/shadow' \
    --archive-repo-spool='/srv/exorg_exsrv/data/.spool/archive-repo' \
    --unarchive-repo-spool='/srv/exorg_exsrv/data/.spool/unarchive-repo' \
    --git-gc-scan-start='20m' \
    --git-gc-scan-every='240h' \
    --stat-author='nogfsostad <admin@example.org>' \
    --stat-scan-start='10m' \
    --stat-scan-every='24h' \
    --init-limit-max-files='2k' \
    --init-limit-max-size='20g' \
    --prefix-init-limit='/srv/exorg_exsrv/data/exsrv/tem-707:2k:250g' \
    --host='storage.example.org' \
    --prefix='/exsrv' \
    --prefix='/exorg' \
    exsrv \
    exorg
```
