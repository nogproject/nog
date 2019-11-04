# Bootstrapping the Unix Domain Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `nogfsoctl` shell, init the Unix domain:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get registries

nogfsoctl init unix-domain --no-vid EXO

nogfsoctl get unix-domain EXO
```

In the `nog-fso-the-hard-way-k8s` workspace:

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsodomd-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsodomd-etc/fso-tls-cabundle.pem
cp local/pki/tls/nogfsodomd-combined.pem local/k8s/secrets/nogfsodomd-etc/fso-tls-combined.pem
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-stad' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsodomd`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/k8s/secrets/nogfsodomd-etc/nogfsodomd.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsodomd+storage",
    "aud": ["fso"],
    "scopes": [
        {
            "actions": [
                "uxd/read-unix-domain",
                "uxd/write-unix-domain"
            ],
            "names": [
                "EXO"
            ]
        }
    ]
}
EOF
```

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsodomd-etc && \
    kubectl create secret generic \
        nogfsodomd-etc $(printf -- '--from-file=%s ' *.pem *.jwt) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Update the `storage` stateful set to start `nogfsodomd`:

```
 ngfstaUid=$(docker run --rm ${nogfsostoLocalImage} id -u ngfsta) &&
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
        - name: nogfsostad-run
          emptyDir: {}
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
        - name: nogfsorstd-etc
          secret:
            secretName: nogfsorstd-etc
            defaultMode: 0444
        - name: nogfsorstd-var-lib
          emptyDir: {}
        - name: nogfsodomd-etc
          secret:
            secretName: nogfsodomd-etc
            defaultMode: 0444
      initContainers:
        - name: init
          image: ${nogfsostoLocalImage}
          volumeMounts:
            - name: nogfsostad-run
              mountPath: /run/nogfsostad
            - name: nogfsotard-var-lib
              mountPath: /var/lib/nogfsotard
            - name: nogfsosdwbakd3-var-lib
              mountPath: /var/lib/nogfsosdwbakd3
            - name: nogfsorstd-var-lib
              mountPath: /var/lib/nogfsorstd
          args:
            - 'bash'
            - '-c'
            - |
              set -o nounset -o errexit -o pipefail -o noglob
              set -x
              :
              install -m 0750 -o ngfsta -g ngfsta -d /run/nogfsostad/sududod
              :
              install -m 0755 -o ngftar -g ngftar -d /var/lib/nogfsotard
              install -m 0700 -o ngftar -g ngftar -d /var/lib/nogfsotard/.gnupg
              ln -snf /usr/local/etc/nogfsotard/gitconfig /var/lib/nogfsotard/.gitconfig
              :
              install -m 0755 -o ngfbak -g ngfbak -d /var/lib/nogfsosdwbakd3
              install -m 0700 -o ngfbak -g ngfbak -d /var/lib/nogfsosdwbakd3/.gnupg
              :
              install -m 0755 -o ngfrst -g ngfsta -d /var/lib/nogfsorstd
              install -m 0700 -o ngfrst -g ngfsta -d /var/lib/nogfsorstd/.gnupg
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
            - name: nogfsostad-run
              mountPath: /run/nogfsostad
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
        - name: nogfsostasududod
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsostad-run
              mountPath: /run/nogfsostad
            - name: online
              mountPath: /srv/exorg_exsrv
          securityContext:
            capabilities:
              add:
                - LINUX_IMMUTABLE
          args:
            - 'chroot'
            - '--userspec=ngfsta'
            - '/'
            - 'env'
            - 'HOME=/'
            - 'nogfsostasududod'
            - '--shutdown-timeout=40s'
            - '--sududod-socket=/run/nogfsostad/sududod/sock'
            - '--stad-uids=${ngfstaUid}'
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
        - name: nogfsotargctd
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
          args:
            - 'chroot'
            - '--userspec=ngftar'
            - '/'
            - 'env'
            - 'HOME=/var/lib/nogfsotard'
            - 'GIT_ALLOW_PROTOCOL=file:ext'
            - 'nogfsotargctd'
            - '--config'
            - '/usr/local/etc/nogfsotard/nogfsotargctdconfig.sh'
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
        - name: nogfsorstd
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsorstd-etc
              mountPath: /usr/local/etc/nogfsorstd
              readOnly: true
            - name: online
              mountPath: /srv/exorg_exsrv
            - name: tape
              mountPath: /srv/samfs
              readOnly: true
            - name: nogfsorstd-var-lib
              mountPath: /var/lib/nogfsorstd
          securityContext:
            capabilities:
              add:
                - CHOWN
                - DAC_OVERRIDE
                - FOWNER
          args:
            - 'chroot'
            - '--userspec=ngfrst'
            - '/'
            - 'env'
            - 'HOME=/var/lib/nogfsorstd'
            - 'nogfsorstd'
            - '--nogfsoregd=fso.default.svc.cluster.local:7550'
            - '--tls-cert=/usr/local/etc/nogfsorstd/fso-tls-combined.pem'
            - '--tls-ca=/usr/local/etc/nogfsorstd/fso-tls-cabundle.pem'
            - '--sys-jwt=/usr/local/etc/nogfsorstd/nogfsorstd.jwt'
            - '--cap-path=/usr/lib/nogfsorstd'
            - '--shutdown-timeout=40s'
            - '--log=prod'
            - '--host=storage.example.org'
            - '--prefix=/exsrv'
            - '--prefix=/exorg'
            - 'exsrv'
            - 'exorg'
        - name: nogfsodomd
          image: '${nogfsostoLocalImage}'
          volumeMounts:
            - name: nogfsodomd-etc
              mountPath: /usr/local/etc/nogfsodomd
              readOnly: true
          args:
            - 'chroot'
            - '--userspec=ngfdom'
            - '/'
            - 'env'
            - 'HOME=/'
            - 'nogfsodomd'
            - '--nogfsoregd=fso.default.svc.cluster.local:7550'
            - '--tls-cert=/usr/local/etc/nogfsodomd/fso-tls-combined.pem'
            - '--tls-ca=/usr/local/etc/nogfsodomd/fso-tls-cabundle.pem'
            - '--sys-jwt=/usr/local/etc/nogfsodomd/nogfsodomd.jwt'
            - '--sync-domain-start=5s'
            - '--sync-domain-every=30s'
            - '--shutdown-timeout=40s'
            - '--log=prod'
            - '--group-prefix=exsrv_'
            - '--group-prefix=exorg_'
            - 'EXO'
EOF
```

Wait for the first sync to complete:

```bash
kubectl get pods -w
kubectl logs storage-0 -c nogfsodomd -f
```

In the `nogfsoctl` shell, inspect the Unix domain:

```bash
nogfsoctl get unix-domain EXO
```

Add a placeholder user that represents your Gitimp user that you used to log in
to the web application:

```bash
user=...

kubectl exec storage-0 -c nogfsodomd -- adduser --system --ingroup exorg_ag-alice "${user}"
```

Wait for the next sync (up to 30s):

```bash
kubectl logs storage-0 -c nogfsodomd -f
```

Then reload the web application, inspecting the log to confirm that the web
application updates the user group information:

```bash
kubectl logs deployment/nog-app-2 -f
```

You can also inspect the user in MongoDB:

```
kubectl attach -it deployments/mongo
rs0:PRIMARY> use nog
rs0:PRIMARY> db.users.findOne()
```
