# Bootstrapping the Unix Domain Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `nogfsoctl` shell, init the Unix domain:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get registries

nogfsoctl init unix-domain --no-vid EXO

nogfsoctl get unix-domain EXO
```

In the `nog-fso-the-hard-way-systemd` workspace:

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
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
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/pki/jwt/nogfsodomd.jwt
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

Install certificates and JWT:

```bash
vagrant ssh storage -- sudo \
    install -m 0755 </dev/null -d /etc/nogfsodomd

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/tls/ca.pem /dev/stdin /etc/nogfsodomd/fso-tls-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfdom <local/pki/tls/nogfsodomd-combined.pem /dev/stdin /etc/nogfsodomd/fso-tls-combined.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfdom <local/pki/jwt/nogfsodomd.jwt /dev/stdin /etc/nogfsodomd/nogfsodomd.jwt
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsodomd.service
[Unit]
Description=nogfsodomd

[Service]
Restart=always
User=ngfdom
Environment='HOME=/'
ExecStart=\
    '/usr/bin/nogfsodomd' \
        '--nogfsoregd=fso.example.org:7550' \
        '--tls-cert=/etc/nogfsodomd/fso-tls-combined.pem' \
        '--tls-ca=/etc/nogfsodomd/fso-tls-cabundle.pem' \
        '--sys-jwt=/etc/nogfsodomd/nogfsodomd.jwt' \
        '--sync-domain-start=5s' \
        '--sync-domain-every=30s' \
        '--shutdown-timeout=40s' \
        '--log=prod' \
        '--group-prefix=exsrv_' \
        '--group-prefix=exorg_' \
        'EXO' \
    ;

ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes
ProtectKernelModules=yes
PrivateDevices=yes
SystemCallArchitectures=native
NoNewPrivileges=yes
CapabilityBoundingSet=

[Install]
WantedBy=default.target
EOF
```

Start the `nogfsodomd` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsodomd
systemctl start nogfsodomd
systemctl status nogfsodomd
EOF
```

Inspect the log, waiting for the first sync to complete:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsodomd -f
```

In the `nogfsoctl` shell, inspect the Unix domain:

```bash
kubectl attach -it deployments/nogfsoctl

nogfsoctl get unix-domain EXO
```

Add a placeholder user that represents your Gitimp user that you used to log in
to the web application:

```bash
user=...

vagrant ssh storage -- sudo adduser --system --ingroup exorg_ag-alice "${user}"
```

Wait for the next sync (up to 30s):

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsodomd -f
```

Then reload the web application, inspecting the log to confirm that the web
application updates the user group information:

```bash
kubectl logs deployment/nog-app-2 -f
```

You can also inspect the user in MongoDB:

```
kubectl attach -it deployments/mongo
rs0:PRIMARY> use nog
rs0:PRIMARY> db.users.findOne()
```
