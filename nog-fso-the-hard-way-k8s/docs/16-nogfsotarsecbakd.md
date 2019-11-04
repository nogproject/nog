# Bootstrapping the Tar Secrets Backup Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `storage-shell`:

```bash
kubectl attach -it statefulset.apps/storage
```

Create the data directory:

```bash
install -m 0750 -o ngftar -g ngftar -d /srv/samfs/tartt-secrets-backup
```

In the `nog-fso-the-hard-way-k8s` workspace:

Create a Kubernetes config map:

```bash
mkdir -p local/k8s/configmaps/nogfsotarsecbakd-etc
```

Configure `nogfsotarsecbakd`:

```bash
 cat <<\EOF >local/k8s/configmaps/nogfsotarsecbakd-etc/nogfsotarsecbakdconfig.sh
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

Create the config map:

```bash
kubectl create configmap nogfsotarsecbakd-etc --from-file=local/k8s/configmaps/nogfsotarsecbakd-etc
```

Update the `storage` stateful set to start `nogfsotarsecbakd`:

```
 kubectl apply -f - <<EOF
kind: StatefulSet
apiVersion: apps/v1
metadata:
  name: storage
spec:
  serviceName: storage
  replicas: 1
  selector:
    matchLabels:
      app: storage
  template:
    metadata:
      labels:
        app: storage
    spec:
      volumes:
        - name: online
          persistentVolumeClaim:
            claimName: storage-online
        - name: tape
          persistentVolumeClaim:
            claimName: storage-tape
        - name: nogfsostad-etc
          secret:
            secretName: nogfsostad-etc
            defaultMode: 0444
        - name: nogfsotard-etc
          secret:
            secretName: nogfsotard-etc
            defaultMode: 0444
        - name: nogfsotard-var-lib
          emptyDir: {}
        - name: nogfsotarsecbakd-etc
          configMap:
            name: nogfsotarsecbakd-etc
            defaultMode: 0444
        - name: nogfsosdwbakd3-etc
          secret:
            secretName: nogfsosdwbakd3-etc
            defaultMode: 0444
        - name: nogfsosdwbakd3-var-lib
          emptyDir: {}
      initContainers:
        - name: init
          image: ${nogfsostoLocalImage}
          volumeMounts:
            - name: nogfsotard-var-lib
              mountPath: /var/lib/nogfsotard
            - name: nogfsosdwbakd3-var-lib
              mountPath: /var/lib/nogfsosdwbakd3
          args:
            - 'bash'
            - '-c'
            - |
              set -o nounset -o errexit -o pipefail -o noglob
              set -x
              :
              install -m 0755 -o ngftar -g ngftar -d /var/lib/nogfsotard
              install -m 0700 -o ngftar -g ngftar -d /var/lib/nogfsotard/.gnupg
              ln -snf /usr/local/etc/nogfsotard/gitconfig /var/lib/nogfsotard/.gitconfig
              :
              install -m 0755 -o ngfbak -g ngfbak -d /var/lib/nogfsosdwbakd3
              install -m 0700 -o ngfbak -g ngfbak -d /var/lib/nogfsosdwbakd3/.gnupg
      containers:
        - name: storage-shell
          image: ${nogfsostoLocalImage}
          stdin: true
          tty: true
          volumeMounts:
            - name: online
              mountPath: /srv/exorg_exsrv
            - name: tape
              mountPath: /srv/samfs
            - name: nogfsotard-var-lib
              mountPath: /var/lib/nogfsotard
            - name: nogfsosdwbakd3-var-lib
              mountPath: /var/lib/nogfsosdwbakd3
        - name: nogfsostad
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsostad-etc
              mountPath: /usr/local/etc/nogfsostad
              readOnly: true
            - name: online
              mountPath: /srv/exorg_exsrv
          args:
            - 'env'
            - 'HOME=/'
            - 'nogfsostasvsd'
            - '--userspec=ngfsta'
            - '--group-prefix=ag_exorg'
            - '--group-prefix=exsrv_'
            - '--group-prefix=exorg_'
            - '--'
            - 'nogfsostad'
            - '--nogfsoregd=fso.default.svc.cluster.local:7550'
            - '--tls-cert=/usr/local/etc/nogfsostad/fso-tls-combined.pem'
            - '--tls-ca=/usr/local/etc/nogfsostad/fso-tls-cabundle.pem'
            - '--jwt-ca=/usr/local/etc/nogfsostad/fso-jwt-cabundle.pem'
            - '--jwt-ou=nog-jwt'
            - '--jwt-unix-domain=EXO'
            - '--sys-jwt=/usr/local/etc/nogfsostad/nogfsostad.jwt'
            - '--session-name=storage.example.org'
            - '--git-fso-program=/usr/bin/git-fso'
            - '--sududod-socket=/run/nogfsostad/sududod/sock'
            - '--shutdown-timeout=40s'
            - '--log=prod'
            - '--gitlab=no'
            - '--git-committer=nogfsostad <admin@example.org>'
            - '--trim-host-root=/srv/exorg_exsrv/data'
            - '--shadow-root=/srv/exorg_exsrv/shadow'
            - '--archive-repo-spool=/srv/exorg_exsrv/data/.spool/archive-repo'
            - '--unarchive-repo-spool=/srv/exorg_exsrv/data/.spool/unarchive-repo'
            - '--git-gc-scan-start=20m'
            - '--git-gc-scan-every=240h'
            - '--stat-author=nogfsostad <admin@example.org>'
            - '--stat-scan-start=10m'
            - '--stat-scan-every=24h'
            - '--init-limit-max-files=2k'
            - '--init-limit-max-size=20g'
            - '--prefix-init-limit=/srv/exorg_exsrv/data/exsrv/tem-707:2k:250g'
            - '--host=storage.example.org'
            - '--prefix=/exsrv'
            - '--prefix=/exorg'
            - 'exsrv'
            - 'exorg'
        - name: nogfsotard
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsotard-etc
              mountPath: /usr/local/etc/nogfsotard
              readOnly: true
            - name: online
              mountPath: /srv/exorg_exsrv
            - name: tape
              mountPath: /srv/samfs
            - name: nogfsotard-var-lib
              mountPath: /var/lib/nogfsotard
          securityContext:
            capabilities:
              add:
                - DAC_READ_SEARCH
          args:
            - 'chroot'
            - '--userspec=ngftar'
            - '/'
            - 'env'
            - 'HOME=/var/lib/nogfsotard'
            - 'GIT_ALLOW_PROTOCOL=file:ext'
            - 'nogfsotard'
            - '--config'
            - '/usr/local/etc/nogfsotard/nogfsotardconfig.sh'
        - name: nogfsotarsecbakd
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsotarsecbakd-etc
              mountPath: /usr/local/etc/nogfsotarsecbakd
              readOnly: true
            - name: online
              mountPath: /srv/exorg_exsrv
              readOnly: true
            - name: tape
              mountPath: /srv/samfs
          args:
            - 'chroot'
            - '--userspec=ngftar'
            - '/'
            - 'env'
            - 'HOME=/'
            - 'nogfsotarsecbakd'
            - '--config'
            - '/usr/local/etc/nogfsotarsecbakd/nogfsotarsecbakdconfig.sh'
        - name: nogfsosdwbakd3
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsosdwbakd3-etc
              mountPath: /usr/local/etc/nogfsosdwbakd3
              readOnly: true
            - name: online
              mountPath: /srv/exorg_exsrv
              readOnly: true
            - name: tape
              mountPath: /srv/samfs
            - name: nogfsosdwbakd3-var-lib
              mountPath: /var/lib/nogfsosdwbakd3
          securityContext:
            capabilities:
              add:
                - DAC_READ_SEARCH
          args:
            - 'chroot'
            - '--userspec=ngfbak'
            - '/'
            - 'env'
            - 'HOME=/var/lib/nogfsosdwbakd3'
            - 'nogfsosdwbakd3'
            - '--config'
            - '/usr/local/etc/nogfsosdwbakd3/nogfsosdwbakd3config.sh'
EOF
```

Check the new daemon's log:

```bash
kubectl get pods -w
kubectl logs storage-0 -c nogfsotarsecbakd
```
