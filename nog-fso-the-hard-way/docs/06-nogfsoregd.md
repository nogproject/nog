# Bootstrapping the FSO Registry Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On host `fso.example.org`:

Open <http://localhost:8080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-regd' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsoregd`:

```bash
NOG_JWT="eyJ..."

curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://nog.example.org:8080/api/v1/fso/sysauth \
| jq -r .data.token | tee /host/local/pki/jwt/nogfsoregd.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsoregd",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "allaggsig" },
        {
            "actions": [
                "fso/read-registry",
                "fso/exec-ping-registry"
            ],
            "names": [
                "exsrv",
                "exorg"
            ]
        },
        {
            "actions": [
                "fso/read-root",
                "fso/read-repo",
                "fso/exec-split-root",
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

To inspect the JWT content, paste the token into <https://jwt.io>.

Add a daemon user:

```bash
adduser --system --group ngfreg
```

Start MongoDB:

```bash
apt-get install -y mongodb

/etc/init.d/mongodb start
/etc/init.d/mongodb status
pgrep -fa mongod
```

Install `nogfsoregd`:

```bash
mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2
install -m 0755 /tmp/fso/bin/nogfsoregd /usr/local/bin/nogfsoregd
rm -rf /tmp/fso
```

Install certs:

```bash
install -m 0755 -d /usr/local/etc/nogfsoregd
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nogfsoregd/fso-tls-cabundle.pem
install -m 0640 -g ngfreg /host/local/pki/tls/nogfsoregd-combined.pem /usr/local/etc/nogfsoregd/fso-tls-combined.pem
install -m 0644 /host/local/pki/jwt/ca.pem /usr/local/etc/nogfsoregd/fso-jwt-cabundle.pem
install -m 0640 -g ngfreg /host/local/pki/jwt/nogfsoregd.jwt /usr/local/etc/nogfsoregd/jwt
```

Start `nogfsoregd`:

```bash
chroot --userspec=ngfreg / \
env HOME=/ \
nogfsoregd \
    --log=prod \
    --mongodb=mongodb://localhost/nogfsoreg \
    --shutdown-timeout=40s \
    --bind-grpc=0.0.0.0:7550 \
    --bind-rgrpc=0.0.0.0:7551 \
    --advertise-rgrpc=fso.example.org:7551 \
    --tls-cert=/usr/local/etc/nogfsoregd/fso-tls-combined.pem \
    --tls-ca=/usr/local/etc/nogfsoregd/fso-tls-cabundle.pem \
    --jwt-ca=/usr/local/etc/nogfsoregd/fso-jwt-cabundle.pem \
    --jwt-ou=nog-jwt \
    --proc-registry-jwt=/usr/local/etc/nogfsoregd/jwt \
    --proc-registry=exsrv \
    --proc-registry=exorg \
    --events-gc-scan-start=10s \
    --events-gc-scan-every=1m \
    --history-trim-scan-start=20s \
    --history-trim-scan-every=1m \
    --workflows-gc-scan-start=30s \
    --workflows-gc-scan-every=1m
```
