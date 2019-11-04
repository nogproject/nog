# Bootstrapping the Admin Command Line Tool
By Steffen Prohaska
<!--@@VERSIONINC@@-->

We will use the image that has been set earlier:

```bash
echo "nogfsoctlImage: ${nogfsoctlImage}"
```

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nogfsoctl-etc
cp local/pki/tls/ca.pem local/k8s/secrets/nogfsoctl-etc/ca.pem
cp local/pki/tls/alice-combined.pem local/k8s/secrets/nogfsoctl-etc/cert-combined.pem
```

Open <http://localhost:30080> in Chrome, and issue an admin JWT by executing
the following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/nogfsoctl-admin' }, console.log);
```

Copy the token, and add it to the secret files:

```bash
 NOG_JWT="eyJ..."

tr -d '"' <<<"${NOG_JWT}" | tee local/k8s/secrets/nogfsoctl-etc/nogfsoctl.jwt
unset NOG_JWT
```

Create the secret:

```bash
(
    cd local/k8s/secrets/nogfsoctl-etc && \
    kubectl create secret generic --dry-run -o yaml \
        nogfsoctl-etc $(printf -- '--from-file=%s ' *.pem *.jwt)
) \
| kubectl apply -f -
```

Deploy `nogfsoctl`:

```
 kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nogfsoctl
  namespace: default
  labels:
    app: nogfsoctl
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nogfsoctl
  template:
    metadata:
      labels:
        app: nogfsoctl
    spec:
      volumes:
        - name: nogfsoctl-etc
          secret:
            secretName: nogfsoctl-etc
            defaultMode: 0444
      containers:
        - name: nogfsoctl
          image: ${nogfsoctlImage}
          stdin: true
          tty: true
          volumeMounts:
            - name: nogfsoctl-etc
              mountPath: /etc/nogfsoctl
              readOnly: true
EOF
```

Attach `nogfsoctl`:

```bash
kubectl attach -it deployments/nogfsoctl
```

Set an alias for `nogfsoctl`:

```bash
alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get registries
```

To detach, type `<C-P><C-Q>`.
