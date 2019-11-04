# Bootstrapping the Unix Domain Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install `nogfsodomd`:

```bash
mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2
install -m 0755 /tmp/fso/bin/nogfsodomd /usr/local/bin/nogfsodomd
rm -rf /tmp/fso
```

Add a daemon user:

```bash
adduser --system --group ngfdom
```

Configure `nogfsodomd`:

Open <http://localhost:8080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsodomd`:

```bash
NOG_JWT="eyJ..."

curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://nog.example.org:8080/api/v1/fso/sysauth \
| jq -r .data.token | tee /host/local/pki/jwt/nogfsodomd.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsodomd+storage",
    "aud": ["fso"],
    "scopes": [
        {
            "actions": [
                "uxd/read-unix-domain",
                "uxd/write-unix-domain"
            ],
            "names": [
                "EXO"
            ]
        }
    ]
}
EOF
```

Install etc files:

```bash
install -m 0755 -d /usr/local/etc/nogfsodomd
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nogfsodomd/fso-tls-cabundle.pem
install -m 0640 -g ngfdom /host/local/pki/tls/nogfsodomd-combined.pem /usr/local/etc/nogfsodomd/fso-tls-combined.pem
install -m 0640 -g ngfdom /host/local/pki/jwt/nogfsodomd.jwt /usr/local/etc/nogfsodomd/jwt
```

On `ops.example.org`, init the Unix domain:

```bash
nogfsoctl init unix-domain --no-vid EXO
```

On `storage.example.org`, start `nogfsodomd`:

```bash
chroot --userspec=ngfdom / \
env HOME='/' \
nogfsodomd \
    --nogfsoregd=fso.example.org:7550 \
    --tls-cert=/usr/local/etc/nogfsodomd/fso-tls-combined.pem \
    --tls-ca=/usr/local/etc/nogfsodomd/fso-tls-cabundle.pem \
    --sys-jwt=/usr/local/etc/nogfsodomd/jwt \
    --sync-domain-start=5s \
    --shutdown-timeout=40s \
    --log=prod \
    --group-prefix=exsrv_ \
    --group-prefix=exorg_ \
    EXO
```

Wait for the first sync to complete.

On `ops.example.org`, inspect the Unix domain:

```bash
nogfsoctl get unix-domain EXO
```

On `storage.example.org`, add a placeholder user that represents your Gitimp
user that you used to log in to the web application:

```bash
user=...

adduser --system --ingroup exorg_ag-alice ${user}
```

Restart `nogfsdomd`, and wait for the initial sync.  Then reload the web
application, and inspect the output in the `nog.example.org` terminal to
confirm that the web application updates the user group information.  You can
also inspect the user in MongoDB, on `nog.example.org`:

```
$ mongo
> use nog
> db.users.findOne()
```
