# Bootstrapping the Shadow Backup Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `storage-shell`:

```bash
kubectl attach -it statefulset.apps/storage
```

Create the data directory:

```bash
install -m 0750 -o ngfbak -g ngfbak -d /srv/samfs/nogfsobak
```

In the `nog-fso-the-hard-way-k8s` workspace:

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsosdwbakd3-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsosdwbakd3-etc/fso-tls-cabundle.pem
cp local/pki/tls/nogfsosdwbakd3-combined.pem local/k8s/secrets/nogfsosdwbakd3-etc/fso-tls-combined.pem
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsosdwbakd3`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/k8s/secrets/nogfsosdwbakd3-etc/nogfsosdwbakd3.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsosdwbakd3+storage",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "all" },
        { "action": "fso/read-registry", "names": ["exsrv", "exorg"] },
        {
            "actions": [
                "fso/read-repo",
                "fso/init-repo-shadow-backup"
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

Configure `nogfsosdwbakd3`:

```bash
 cat <<\EOF >local/k8s/secrets/nogfsosdwbakd3-etc/nogfsosdwbakd3config.sh
# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='storage.example.org'
# `cfgShadowRoots` lists valid shadow repository path prefixes, one per line.
# Backup Tar archives will contain paths relative to one of the shadow roots.
cfgShadowRoots='
/srv/exorg_exsrv/shadow
'

# `cfgShadowBackupUrlForRepo <repoId> <globalPath>` prints the URL of the
# backups of the shadow repo `<repoId>`.
#
# The function is only called if the backup path is not yet stored in the
# registry.
cfgShadowBackupUrlForRepo() {
    local id="$1"
    local globalPath="$2"

    local orgUnit
    case ${globalPath} in
    /exsrv/*/*)
        orgUnit="$(cut -d / -f 4 <<<"${globalPath}")"
        ;;
    /exorg/*)
        orgUnit="$(cut -d / -f 3 <<<"${globalPath}")"
        ;;
    *)
        echo >&2 "Unknown global path prefix."
        return 1
        ;;
    esac

    # Mangle the current year into the backup path.
    local year
    year="$(date +%Y)"

    printf \
        'nogfsobak://storage.example.org/srv/samfs/nogfsobak/%s/%s/%s/%s/%s' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}"
}

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before a backup.  Backups are skipped if `df`
# reports less.
cfgCheckMinDf='
/srv/samfs/nogfsobak 10000
'

# `cfgBuckets` is a list of `<bucket> <max> <selector>...`.  The latest
# backup is added to `<bucket>` if `find -type f <selector>` does not match
# the most recent file in the bucket.  The oldest backups are deleted if a
# bucket contains more than `<max>` files.
#
# To ensure that the latest state is always in at least one bucket, bucket
# `latest` uses `-false`, so that it receives every backup.
cfgBuckets='
latest 2 -false
hourly 10 -mmin -60
daily 7 -mmin -1440
weekly 5 -mtime -7
monthly 4 -mtime -30
'

# `cfgCapPath` contains programs with capabilities.  See `Dockerfile.jnj`.
cfgCapPath='/usr/lib/nogfsosdwbakd3'

# `cfgPrivGitForEachRef` controls how to run `git for-each-ref`.
#
#  - `none`: Use `git`.
#  - `sudo`: Use sudo to determine the owner of the repo and then use sudo to
#    run `git for-each-ref` as that user.
#  - `cap`: Use `git` from `cfgCapPath`.
#
cfgPrivGitForEachRef='sudo'

# `cfgNogfsoschdShadowBackup` defines the `nogfsoschd` command and arguments
# that control when to back up a shadow repo.  Here:
#
#  - process repos for Nogfsostad registries and prefixes;
#  - process a repo on any ref update;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdShadowBackup() {
  nogfsoschd \
      --nogfsoregd='fso.default.svc.cluster.local:7550' \
      --tls-cert='/usr/local/etc/nogfsosdwbakd3/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsosdwbakd3/fso-tls-cabundle.pem' \
      --sys-jwt='/usr/local/etc/nogfsosdwbakd3/nogfsosdwbakd3.jwt' \
      --state='/var/lib/nogfsosdwbakd3' \
      --scan-start \
      --scan-every='1h' \
      --host='storage.example.org' \
      --registry='exsrv' \
      --registry='exorg' \
      --prefix='/exsrv' \
      --prefix='/exorg' \
      "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact
# with the registry, specifically to `nogfsoctl init-shadow-backup`.
cfgNogfsoctl() {
  nogfsoctl \
      --nogfsoregd='fso.default.svc.cluster.local:7550' \
      --tls-cert='/usr/local/etc/nogfsosdwbakd3/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsosdwbakd3/fso-tls-cabundle.pem' \
      --jwt-auth=no --jwt='/usr/local/etc/nogfsosdwbakd3/nogfsosdwbakd3.jwt' \
      "$@"
}
EOF
```

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsosdwbakd3-etc && \
    kubectl create secret generic \
        nogfsosdwbakd3-etc $(printf -- '--from-file=%s ' *.pem *.jwt *.sh) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Update the `storage` stateful set to start `nogfsosdwbakd3`:

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
kubectl logs storage-0 -c nogfsosdwbakd3
```
