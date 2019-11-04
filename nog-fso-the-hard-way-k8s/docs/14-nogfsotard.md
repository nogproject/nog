# Bootstrapping the Tar Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `storage-shell`:

```bash
kubectl attach -it statefulset.apps/storage
```

Create data directories:

```bash
install -m 0750 -o ngftar -g ngftar -d /srv/exorg_exsrv/tartt
install -m 0750 -o ngftar -g ngftar -d /srv/samfs/tartt-tars
```

In the `nog-fso-the-hard-way-k8s` workspace:

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsotard-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsotard-etc/fso-tls-cabundle.pem
cp local/pki/tls/nogfsotard-combined.pem local/k8s/secrets/nogfsotard-etc/fso-tls-combined.pem
```

Add a Git configuration:

```bash
cat <<\EOF >local/k8s/secrets/nogfsotard-etc/gitconfig
[user]
    name = tar daemon
    email = admin@example.org
EOF
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsotard`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/k8s/secrets/nogfsotard-etc/nogfsotard.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsotard+storage",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "all" },
        { "action": "fso/read-registry", "names": ["exsrv", "exorg"] },
        {
            "actions": [
                "fso/read-repo",
                "fso/init-repo-tartt"
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

Configure `nogfsotard`:

```bash
 cat <<\EOF >local/k8s/secrets/nogfsotard-etc/nogfsotardconfig.sh
# `cfgTarttUrlForRepo <repoId> <globalPath> <realdir>` prints the URL of the
# tartt repo used for the realdir archives of repo `<repoId>`.
#
# The function is only called if the URL is not yet stored in the registry.
cfgTarttUrlForRepo() {
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

    # Mangle the current year into the tar path.
    local year
    year="$(date +%Y)"

    local host='storage.example.org'
    local path="$(printf \
        '/srv/exorg_exsrv/tartt/%s/%s/%s/%s/%s.tartt' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    local tardir="$(printf \
        '/srv/samfs/tartt-tars/%s/%s/%s/%s/%s.tars' \
        "${year}" "${orgUnit}" "${id:0:2}" "${id:2:2}" "${id}" \
    )"
    printf 'tartt://%s%s?driver=localtape&tardir=%s' \
        "${host}" "${path}" "${tardir}"
}

# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='storage.example.org'
# `cfgShadowRoots` lists valid shadow path prefixes, one per line.  Repos that
# use a different prefix are not processed.
cfgShadowRoots='
/srv/exorg_exsrv/shadow
'

# `cfgTarttStoreName` controls the Tartt store name.
cfgTarttStoreName='storage'

# `cfgTarEncryption` controls whether archives are encrypted.  Valid values:
#
#  - `none`: use `tartt tar --insecure-plaintext`
#  - `gpg`: use `tartt tar --recipient=...` or `tartt tar --plaintext-secret`
#
cfgTarEncryption='gpg'

# `cfgCheckMinDf` are lines `<path> <minDf>` that specify the required free
# disk space in 1k df blocks before starting an archive.  Archives are skipped
# if `df` reports less.
cfgCheckMinDf='
/srv/exorg_exsrv/tartt 10000
/srv/samfs/tartt-tars 10000
'

# `cfgBandwidthLimit` limits the data that is written per second.  It must be
# specified with a suffix `M` to indicate Megabytes per second.
cfgBandwidthLimit='60M'

# Set `cfgFakeArchives=t` to replace tar files with placeholders, which may
# be useful for testing.
cfgFakeArchives=

# To access shadow repos:
#
#  - use Sudo for Git read access;
#  - use capabilities for Tar read access.
#
# for detailed documentation.
cfgPrivGitRemote='sudo'
cfgPrivGitForEachRef='sudo'
cfgPrivGitArchive='sudo'
cfgCapPath='/usr/lib/nogfsotard'

# `cfgNogfsoschdTartt` defines the `nogfsoschd` command and arguments
# that control when to run tartt for realdirs.  Here:
#
#  - process repos with certain prefixes in certain registries;
#  - process a repo when `master-stat` changes;
#  - scan during start and every hour as a fallback if ref updates get lost.
#
cfgNogfsoschdTartt() {
  nogfsoschd \
      --nogfsoregd='fso.default.svc.cluster.local:7550' \
      --tls-cert='/usr/local/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsotard/fso-tls-cabundle.pem' \
      --sys-jwt='/usr/local/etc/nogfsotard/nogfsotard.jwt' \
      --state='/var/lib/nogfsotard' \
      --scan-start \
      --scan-every='24h' \
      --host='storage.example.org' \
      --ref=refs/heads/master-stat \
      --registry='exsrv' \
      --registry='exorg' \
      --prefix='/exsrv' \
      --prefix='/exorg' \
      "$@"
}

# `cfgNogfsoctl` defines the `nogfsoctl` command and arguments to interact with
# the registry, specifically to `nogfsoctl init-tartt`.
cfgNogfsoctl() {
  nogfsoctl \
      --nogfsoregd='fso.default.svc.cluster.local:7550' \
      --tls-cert='/usr/local/etc/nogfsotard/fso-tls-combined.pem' \
      --tls-ca='/usr/local/etc/nogfsotard/fso-tls-cabundle.pem' \
      --jwt-auth=no --jwt='/usr/local/etc/nogfsotard/nogfsotard.jwt' \
      "$@"
}
EOF
```

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsotard-etc && \
    kubectl create secret generic \
        nogfsotard-etc $(printf -- '--from-file=%s ' *.pem *.jwt *.sh gitconfig) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Update the `storage` stateful set to start `nogfsotard`:

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
      initContainers:
        - name: init
          image: ${nogfsostoLocalImage}
          volumeMounts:
            - name: nogfsotard-var-lib
              mountPath: /var/lib/nogfsotard
          args:
            - 'bash'
            - '-c'
            - |
              set -o nounset -o errexit -o pipefail -o noglob
              set -x
              install -m 0755 -o ngftar -g ngftar -d /var/lib/nogfsotard
              install -m 0700 -o ngftar -g ngftar -d /var/lib/nogfsotard/.gnupg
              ln -snf /usr/local/etc/nogfsotard/gitconfig /var/lib/nogfsotard/.gitconfig
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
EOF
```

Check the new daemon's log:

```bash
kubectl get pods -w
kubectl logs storage-0 -c nogfsotard
```
