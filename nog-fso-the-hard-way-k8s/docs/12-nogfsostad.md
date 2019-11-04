# Bootstrapping the Main File Server Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `storage-shell`:

```bash
kubectl attach -it statefulset.apps/storage
```

Initialize the shadow toplevel directories:

```bash
install -m u=rwx,g=rxs,o= -g ngfsta -d /srv/exorg_exsrv/shadow

find /srv/exorg_exsrv/data -mindepth 1 -maxdepth 2 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=rxs,o=- -g ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '/srv/exorg_exsrv/shadow/%P\0' \
| xargs -0 --verbose -n 1 install -m u=rwx,g=s,o=- -o ngfsta -d

find /srv/exorg_exsrv/data -mindepth 3 -maxdepth 3 -type d -printf '%P\0' \
| xargs -0 --verbose -n 1 -i chgrp '--reference=/srv/exorg_exsrv/data/{}' '/srv/exorg_exsrv/shadow/{}'

find /srv/exorg_exsrv/shadow/ -ls
```

Initialize the spool directories:

```bash
install -m 0755 -d /srv/exorg_exsrv/data/.spool
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/archive-repo
install -m 0770 -o ngfsta -g ngfsta -d /srv/exorg_exsrv/data/.spool/unarchive-repo

find /srv/exorg_exsrv/data/.spool/ -ls
```

In the `nog-fso-the-hard-way-k8s` workspace:

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsostad-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsostad-etc/fso-tls-cabundle.pem
cp local/pki/tls/nogfsostad-combined.pem local/k8s/secrets/nogfsostad-etc/fso-tls-combined.pem
cp local/pki/jwt/ca.pem local/k8s/secrets/nogfsostad-etc/fso-jwt-cabundle.pem
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsostad`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/k8s/secrets/nogfsostad-etc/nogfsostad.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsostad+storage",
    "san": ["DNS:storage.example.org"],
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/write", "name": "all" },
        { "action": "bc/read", "name": "allaggsig" },
        { "action": "fso/session", "name": "storage.example.org" },
        {
            "actions": [
                "fso/read-registry",
                "fso/exec-ping-registry"
            ],
            "names": ["exsrv", "exorg"]
        },
        {
            "actions": [
                "fso/read-root",
                "fso/exec-du",
                "fso/exec-split-root",
                "fso/read-repo",
                "fso/confirm-repo",
                "fso/exec-freeze-repo",
                "fso/exec-unfreeze-repo",
                "fso/exec-archive-repo",
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

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsostad-etc && \
    kubectl create secret generic \
        nogfsostad-etc $(printf -- '--from-file=%s ' *.pem *.jwt) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Update the `storage` stateful set to start `nogfsostad`:

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
EOF
```

Check the new daemon's log:

```bash
kubectl get pods -w
kubectl logs storage-0 -c nogfsostad
```
