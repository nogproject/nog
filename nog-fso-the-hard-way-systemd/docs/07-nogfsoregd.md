# Bootstrapping the FSO Registry Daemon
By Steffen Prohaska
<!--@@VERSIONINC@@-->

We will use the image that has been set earlier:

```bash
echo "nogfsoregdImage: ${nogfsoregdImage}"
```

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsoregd-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsoregd-etc/fso-tls-cabundle.pem
cp local/pki/tls/nogfsoregd-combined.pem local/k8s/secrets/nogfsoregd-etc/fso-tls-combined.pem
cp local/pki/mongo/ca.pem local/k8s/secrets/nogfsoregd-etc/mongo-cabundle.pem
cp local/pki/mongo/nogfsoregd-combined.pem local/k8s/secrets/nogfsoregd-etc/mongo-combined.pem
cp local/pki/jwt/ca.pem local/k8s/secrets/nogfsoregd-etc/fso-jwt-cabundle.pem
```

Open <http://localhost:30080> in Chrome, and issue a temporary JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/issue-jwts-regd' }, console.log);
```

Copy the token, and use it to issue a JWT for `nogfsoregd`:

```bash
 NOG_JWT="eyJ..."

 curl -X POST \
    -H "Authorization: Bearer ${NOG_JWT}" \
    -H "Content-Type: application/json" \
    -d @- <<EOF \
    http://localhost:30080/api/v1/fso/sysauth \
| jq -r .data.token | tee local/k8s/secrets/nogfsoregd-etc/nogfsoregd.jwt
{
    "expiresIn": 5443200,
    "subuser": "nogfsoregd",
    "aud": ["fso"],
    "scopes": [
        { "action": "bc/read", "name": "allaggsig" },
        {
            "actions": [
                "fso/read-registry",
                "fso/exec-ping-registry"
            ],
            "names": [
                "exsrv",
                "exorg"
            ]
        },
        {
            "actions": [
                "fso/read-root",
                "fso/read-repo",
                "fso/exec-split-root",
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

To inspect the JWT content, paste the token into <https://jwt.io>.

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsoregd-etc && \
    kubectl create secret generic \
        nogfsoregd-etc $(printf -- '--from-file=%s ' *.pem *.jwt) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Create a Kubernetes Service and StatefulSet:

```
 kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: fso
  labels:
    app: fso
spec:
  selector:
    app: nogfsoregd
  ports:
    - port: 7550
      targetPort: 7550
      name: nogfsoregd
    - port: 7551
      targetPort: 7551
      name: nogfsoregd-rgrpc
  clusterIP: None
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nogfsoregd
spec:
  serviceName: fso
  replicas: 1
  selector:
    matchLabels:
      app: nogfsoregd
  template:
    metadata:
      labels:
        app: nogfsoregd
    spec:
      volumes:
        - name: nogfsoregd-etc
          secret:
            secretName: nogfsoregd-etc
            defaultMode: 0444
      containers:
        - name: nogfsoregd
          image: '${nogfsoregdImage}'
          args:
            - 'chroot'
            - '--userspec=ngfreg'
            - '/'
            - 'env'
            - 'HOME=/'
            - 'nogfsoregd'
            - '--log=prod'
            - '--mongodb=mongodb://mongod-0.mongodb.default.svc.cluster.local:27017/fso'
            - '--mongodb-ca=/usr/local/etc/nogfsoregd/mongo-cabundle.pem'
            - '--mongodb-cert=/usr/local/etc/nogfsoregd/mongo-combined.pem'
            - '--shutdown-timeout=40s'
            - '--bind-grpc=0.0.0.0:7550'
            - '--bind-rgrpc=0.0.0.0:7551'
            - '--advertise-rgrpc=fso.example.org:7551'
            - '--tls-cert=/usr/local/etc/nogfsoregd/fso-tls-combined.pem'
            - '--tls-ca=/usr/local/etc/nogfsoregd/fso-tls-cabundle.pem'
            - '--jwt-ca=/usr/local/etc/nogfsoregd/fso-jwt-cabundle.pem'
            - '--jwt-ou=nog-jwt'
            - '--proc-registry-jwt=/usr/local/etc/nogfsoregd/nogfsoregd.jwt'
            - '--proc-registry=exsrv'
            - '--proc-registry=exorg'
            - '--events-gc-scan-start=10s'
            - '--events-gc-scan-every=1m'
            - '--history-trim-scan-start=20s'
            - '--history-trim-scan-every=1m'
            - '--workflows-gc-scan-start=30s'
            - '--workflows-gc-scan-every=1m'
          ports:
            - containerPort: 7550
            - containerPort: 7551
          volumeMounts:
            - name: nogfsoregd-etc
              mountPath: /usr/local/etc/nogfsoregd
              readOnly: true
EOF
```
