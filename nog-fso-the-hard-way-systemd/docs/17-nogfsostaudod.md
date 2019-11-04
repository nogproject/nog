# Bootstrapping the Sudo Helper Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Allow `nogfsosdwbakd3` to `sudo`:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
sed -i -e '/^#ngf/ s/^#//' /etc/sudoers.d/nogfsostasududod
sed -i -e '/^# Uncomment/d' /etc/sudoers.d/nogfsostasududod
EOF
```

Install a Systemd service unit to run `nogfsostasududod` and expose its Unix
domain socket to `nogfsostad`:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
sed -e "s/{{ *stadUids *}}/$(id -u ngfsta)/g" \
    <<\EOF2 | install -m 0644 /dev/stdin /etc/systemd/system/nogfsostasududod.service
[Unit]
Description=nogfsostasududod

[Service]
Restart=always
User=ngfsta
Environment='HOME=/'
ExecStart=\
    '/usr/bin/nogfsostasududod' \
        '--shutdown-timeout=40s' \
        '--sududod-socket=/run/nogfsostad/sududod/sock' \
        '--stad-uids={{ stadUids }}' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/exorg_exsrv/data
# `sudo` requires a writable `/run`.
TemporaryFileSystem=/run
BindPaths=/run/nogfsostad/sududod
PrivateTmp=yes
ProtectHome=yes
ProtectControlGroups=yes
# Allow `sudo`.
CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE
# Allow binding Unix sockets.
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
# Allow root to manipulate files of other users, in general.
CapabilityBoundingSet=CAP_DAC_OVERRIDE CAP_FOWNER
# Allow root to `chattr`.
CapabilityBoundingSet=CAP_LINUX_IMMUTABLE

# Do not use the following protections, because `sudo` would fail.
NoNewPrivileges=no
ProtectKernelTunables=no
ProtectKernelModules=no
PrivateDevices=no
SystemCallArchitectures=

[Install]
WantedBy=default.target
EOF2
EOF
```

Start the `nogfsostasududod` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsostasududod
systemctl start nogfsostasududod
systemctl status nogfsostasududod
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsostasududod -f
```
