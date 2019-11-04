# Bootstrapping the Tar Secrets Backup Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Create the data directory:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
install -m 0750 -o ngftar -g ngftar -d /srv/samfs/tartt-secrets-backup
EOF
```

Configure `nogfsotarsecbakd`:

```bash
vagrant ssh storage -- sudo \
    install -m 0755 </dev/null -d /etc/nogfsotarsecbakd

 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/nogfsotarsecbakd/nogfsotarsecbakdconfig.sh
# `cfgBackupDir` is the toplevel directory below which `nogfsotarsecbakd`
# creates sub-directories for the origins specified in `cfgOrigins`.
cfgBackupDir='/srv/samfs/tartt-secrets-backup'

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before a backup.  Backups are skipped if `df`
# reports less.
cfgCheckMinDf='
/srv/samfs/tartt-secrets-backup 10000
'

# `cfgInterval` is the sleep interval between backups.
cfgInterval='10m'

# `cfgOrigins` is a list of `<name> <dir> <find-args>...` lines.  `<name>` is
# the subdirectory below `cfgBackupDir` into which to store backups of files in
# `<dir>` that are selected by `find` with `<find-args>`.
cfgOrigins='
tartt-secrets /srv/exorg_exsrv/tartt -name secret -o -name secret.asc
'

# `cfgBuckets` is a list of `<bucket> <max> <selector>...`.  The latest backup
# is added to `<bucket>` if `find -type f <selector>` does not match the most
# recent file in the bucket.  The oldest backups are deleted if a bucket
# contains more than `<max>` files.
#
# To ensure that the latest state is always in at least one bucket, bucket
# `latest` uses `-false`, so that it receives every backup.
cfgBuckets='
latest 2 -false
hourly 10 -mmin -60
daily 7 -mmin -1440
weekly 5 -mtime -7
monthly 1 -mtime -30
'
EOF
```

Install a Systemd service unit:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<\EOF /dev/stdin /etc/systemd/system/nogfsotarsecbakd.service
[Unit]
Description=nogfsotarsecbakd

[Service]
Restart=always
User=ngftar
Environment='HOME=/'
ExecStart=\
    '/usr/bin/nogfsotarsecbakd' \
        '--config' \
        '/etc/nogfsotarsecbakd/nogfsotarsecbakdconfig.sh' \
    ;

ProtectSystem=strict
ReadWritePaths=/srv/samfs/tartt-secrets-backup
PrivateTmp=yes
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

Start the `nogfsotarsecbakd` Systemd service:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
systemctl enable nogfsotarsecbakd
systemctl start nogfsotarsecbakd
systemctl status nogfsotarsecbakd
EOF
```

To inspect the log:

```bash
vagrant ssh storage -- \
    sudo journalctl -u nogfsotarsecbakd -f
```
