# Bootstrapping the Sudo Helper Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install `nogfsostasududod` and related programs:

```bash
apt-get install -y acl sudo

mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2

for f in \
    nogfsostasududod \
    nogfsostaudod-fd \
    nogfsostasuod-fd \
; do \
    install -m 0755 /tmp/fso/bin/${f} /usr/local/bin/${f} ; \
done

rm -rf /tmp/fso
```

Configure Sudo to allow `nogfsostasududod` to start the `*-fd` programs:

* We could use a separate user instead of the generic user `daemon`.

```bash
install -m 0440 <<EOF /dev/stdin /etc/sudoers.d/nogfsostasududod
Defaults:daemon closefrom_override, !pam_session, !pam_setcred
daemon ALL=(%ag_exorg) NOPASSWD: /usr/local/bin/nogfsostaudod-fd
daemon ALL=(root) NOPASSWD: /usr/local/bin/nogfsostasuod-fd
EOF
```

Create the run directory:

```bash
install -m 0750 -o daemon -g ngfsta -d /run/nogfsostad/sududod
```

Start `nogfsostasududod`:

```bash
chroot --userspec=daemon:daemon / \
env HOME=/ \
nogfsostasududod \
    --shutdown-timeout=40s \
    --sududod-socket=/run/nogfsostad/sududod/sock \
    --stad-uids=$(id -u ngfsta)
```
