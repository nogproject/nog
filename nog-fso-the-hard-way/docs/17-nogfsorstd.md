# Bootstrapping the Restore Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install `nogfsorstd` and related programs:

```bash
mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2

for f in \
    nogfsorstd \
    tartt \
    tartt-is-dir \
    tartt-store \
; do \
    install -m 0755 /tmp/fso/bin/${f} /usr/local/bin/${f} ; \
done

rm -rf /tmp/fso
```

Configure helper programs with capabilities:

```bash
install -m 0755 -d /usr/local/lib/nogfsorstd
install -m 0750 -g ngfsta /bin/tar /usr/local/lib/nogfsorstd/tar
setcap cap_chown,cap_dac_override,cap_fowner=ep /usr/local/lib/nogfsorstd/tar
```

Configure `nogfsorstd`:

Open <http://localhost:8080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsorstd`:

```bash
NOG_JWT="eyJ..."

curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://nog.example.org:8080/api/v1/fso/sysauth \
| jq -r .data.token | tee /host/local/pki/jwt/nogfsorstd.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsorstd+storage",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "allaggsig" },
        { "action": "fso/read-registry", "names": ["exsrv", "exorg"] },
        {
            "actions": [
                "fso/read-repo",
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

```bash
install -m 0755 -d /usr/local/etc/nogfsorstd
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nogfsorstd/fso-tls-cabundle.pem
install -m 0640 -g ngfsta /host/local/pki/tls/nogfsorstd-combined.pem /usr/local/etc/nogfsorstd/fso-tls-combined.pem
install -m 0640 -g ngfsta /host/local/pki/jwt/nogfsorstd.jwt /usr/local/etc/nogfsorstd/jwt

ln -s /var/lib/nogfsorstd/gnupg /usr/local/etc/nogfsorstd/.gnupg
```

Create the state directories:

```bash
install -m 0700 -o ngfrst -d /var/lib/nogfsorstd
install -m 0700 -o ngfrst -d /var/lib/nogfsorstd/gnupg
```

Start `nogfsorstd`:

```bash
chroot --userspec=ngfrst / \
env HOME='/usr/local/etc/nogfsorstd' \
nogfsorstd \
    --nogfsoregd='fso.example.org:7550' \
    --tls-cert='/usr/local/etc/nogfsorstd/fso-tls-combined.pem' \
    --tls-ca='/usr/local/etc/nogfsorstd/fso-tls-cabundle.pem' \
    --sys-jwt='/usr/local/etc/nogfsorstd/jwt' \
    --cap-path=/usr/local/lib/nogfsorstd \
    --shutdown-timeout=40s \
    --log=prod \
    --host='storage.example.org' \
    --prefix='/exsrv' \
    --prefix='/exorg' \
    exsrv \
    exorg
```
