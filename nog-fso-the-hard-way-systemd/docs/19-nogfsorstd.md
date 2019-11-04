# Bootstrapping the Restore Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
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
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/pki/jwt/nogfsorstd.jwt
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

Install certificates and JWT:

```bash
vagrant ssh storage -- sudo \
    install -m 0755 </dev/null -d /etc/nogfsorstd

vagrant ssh storage -- sudo \
    install -m 0644 <local/pki/tls/ca.pem /dev/stdin /etc/nogfsorstd/fso-tls-cabundle.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfsta <local/pki/tls/nogfsorstd-combined.pem /dev/stdin /etc/nogfsorstd/fso-tls-combined.pem

vagrant ssh storage -- sudo \
    install -m 0640 -g ngfsta <local/pki/jwt/nogfsorstd.jwt /dev/stdin /etc/nogfsorstd/nogfsorstd.jwt
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsorstd.service
[Unit]
Description=nogfsorstd

[Service]
Restart=always
User=ngfrst
Environment='HOME=/var/lib/nogfsorstd'
ExecStart=\
    '/usr/bin/nogfsorstd' \
        '--nogfsoregd=fso.example.org:7550' \
        '--tls-cert=/etc/nogfsorstd/fso-tls-combined.pem' \
        '--tls-ca=/etc/nogfsorstd/fso-tls-cabundle.pem' \
        '--sys-jwt=/etc/nogfsorstd/nogfsorstd.jwt' \
        '--cap-path=/usr/lib/nogfsorstd' \
        '--shutdown-timeout=40s' \
        '--log=prod' \
        '--host=storage.example.org' \
        '--prefix=/exsrv' \
        '--prefix=/exorg' \
        'exsrv' \
        'exorg' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/exorg_exsrv/data/.spool
# Writable `/var/lib/nogfsorstd`.
StateDirectory=nogfsorstd
PrivateTmp=yes
ProtectHome=yes
ProtectControlGroups=yes
# Allow `/usr/lib/nogfsorstd/tar`.
CapabilityBoundingSet=CAP_CHOWN CAP_DAC_OVERRIDE CAP_FOWNER

# Do not use the following protections, because `tar` would fail.
NoNewPrivileges=no
ProtectKernelTunables=no
ProtectKernelModules=no
PrivateDevices=no
SystemCallArchitectures=

[Install]
WantedBy=default.target
EOF
```

Start the `nogfsorstd` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsorstd
systemctl start nogfsorstd
systemctl status nogfsorstd
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsorstd -f
```
