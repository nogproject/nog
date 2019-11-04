# Bootstrapping the Tar GC Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Configure `nogfsotargctd`:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/nogfsotard/nogfsotargctdconfig.sh
# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='storage.example.org'

# `cfgNogfsoschdTarttGc` defines the `nogfsoschd` command and arguments that
# control when `nogfsotargctd` performs garbage collection on a tartt repo.
# Here:
#
#  - regular scans without watching ref updates.
#
cfgNogfsoschdTarttGc() {
  nogfsoschd \
      --nogfsoregd='fso.example.org:7550' \
      --tls-cert='/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/etc/nogfsotard/fso-tls-cabundle.pem' \
      --sys-jwt='/etc/nogfsotard/nogfsotard.jwt' \
      --no-watch \
      --scan-start \
      --scan-every='24h' \
      --host='storage.example.org' \
      --registry='exsrv' \
      --registry='exorg' \
      --prefix='/exsrv' \
      --prefix='/exorg' \
      "$@"
}
EOF
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsotargctd.service
[Unit]
Description=nogfsotargctd

[Service]
Restart=always
User=ngftar
Environment='HOME=/var/lib/nogfsotard'
Environment='GIT_ALLOW_PROTOCOL=file:ext'
ExecStart=\
    '/usr/bin/nogfsotargctd' \
        '--config' \
        '/etc/nogfsotard/nogfsotargctdconfig.sh' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/exorg_exsrv/shadow
ReadWritePaths=/srv/exorg_exsrv/tartt
ReadWritePaths=/srv/samfs/tartt-tars
# `sudo` requires a writable `/run`.
TemporaryFileSystem=/run
PrivateTmp=yes
ProtectHome=yes
ProtectControlGroups=yes
# Allow `sudo`.
CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE
# Allow root to inspect files to determine directory owners.
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

Start the `nogfsotargctd` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsotargctd
systemctl start nogfsotargctd
systemctl status nogfsotargctd
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsotargctd -f
```
