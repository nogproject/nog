# Bootstrapping the Tar GC Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `storage.example.org`:

Install `nogfsotargctd` and related programs:

```bash
apt-get install -y sudo

mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2

for f in \
    git-receive-pack-get-owner \
    git-receive-pack-sudo-owner \
    nogfsoschd \
    nogfsotargctd \
    tartt \
    tartt-is-dir \
    tartt-store \
; do \
    install -m 0755 /tmp/fso/bin/${f} /usr/local/bin/${f} ; \
done

rm -rf /tmp/fso
```

Configure Sudo to run helper programs:

```bash
install -m 0440 <<\EOF /dev/stdin /etc/sudoers.d/nogfsotargctd
ngftar ALL=(root) NOPASSWD: /usr/local/bin/git-receive-pack-get-owner /*
ngftar ALL=(ngfsta) NOPASSWD: /usr/bin/git-receive-pack
EOF
```

Install etc files:

```bash
install -m 0644 <<\EOF /dev/stdin /usr/local/etc/nogfsotard/nogfsotargctdconfig.sh
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
      --tls-cert='/usr/local/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsotard/fso-tls-cabundle.pem' \
      --sys-jwt='/usr/local/etc/nogfsotard/jwt' \
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

Start `nogfsotargctd`:

```bash
chroot --userspec=ngftar / \
env HOME='/usr/local/etc/nogfsotard' GIT_ALLOW_PROTOCOL=file:ext \
nogfsotargctd --config /usr/local/etc/nogfsotard/nogfsotargctdconfig.sh
```
