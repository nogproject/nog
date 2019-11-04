# Bootstrapping the File Server Pod
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Build a Docker image that contains the file server daemons together with
example users and groups:

```bash
mkdir -p local/images/nogfsosto

 cat <<\EOF >local/images/nogfsosto/add-users
groupAgSuper='ag_exorg'

orgUnits='
ag-alice
ag-bob
ag-charly
em-facility
lm-facility
ms-facility
'

services='
spim-100
spim-222
tem-505
rem-707
ms-data
'

facilities='
em
lm
ms
'

# Lines: <user> <orgUnit> <services>...
users='
alice  ag-alice  rem-707 tem-505
bob    ag-bob    rem-707 tem-505
charly ag-charly rem-707 tem-505
'

addgroup "${groupAgSuper}"

for ou in ${orgUnits}; do
    addgroup "exorg_${ou}"
    adduser --system --shell /bin/bash --ingroup "exorg_${ou}" "${ou}-user"
    adduser "${ou}-user" "${groupAgSuper}"
done

for d in ${services}; do
    addgroup "exsrv_${d}"
done

for f in ${facilities}; do
    addgroup "exsrv_${f}-ops"
done

grep -v '^ *$' <<<"${users}" | while read -r user ou srvs; do
    adduser --system --shell /bin/bash --ingroup "exorg_${ou}" "${user}"
    adduser "${user}" "${groupAgSuper}"
    for s in ${srvs}; do
        adduser "${user}" "exsrv_${s}"
        echo "Added user \`${user}\` to service \`${s}\`."
    done
done
EOF

 cat <<EOF >local/images/nogfsosto/Dockerfile
FROM ${nogfsostoImage}
COPY add-users /tmp/
RUN bash -x /tmp/add-users
RUN set -x && sed -i -e '/^#ngf.*ag_exorg/ s/^#//' /etc/sudoers.d/nogfsostasududod
EOF

nogfsostoLocalImage="nogfsosto:ngfhwk8s-b$(date -u +%s)" && echo "${nogfsostoLocalImage}"
docker build -t "${nogfsostoLocalImage}" local/images/nogfsosto
```

Create two persistent volumes (see note at end of document):

```
 kubectl apply -f - <<EOF
kind: PersistentVolume
apiVersion: v1
metadata:
  name: volume-0
  labels:
    type: local
spec:
  storageClassName: manual
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: "/var/lib/ngfhwk8s/volume-0"
---
kind: PersistentVolume
apiVersion: v1
metadata:
  name: volume-1
  labels:
    type: local
spec:
  storageClassName: manual
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: "/var/lib/ngfhwk8s/volume-1"
EOF
```

Claim the persistent volumes:

```bash
 kubectl apply -f - <<EOF
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: storage-online
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: storage-tape
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
```

Deploy the file server simulation pod:

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
EOF
```

Attach the `storage-shell` container:

```bash
kubectl attach -it statefulset.apps/storage
```

To detach, type `<C-P><C-Q>`.

## Note on persistent volumes

The persistent volume setup works with Kubenetes on Docker Desktop for Mac.
The directories are created on the root ext4 filesystem of the Docker VM when
deploying the stateful set.  The directories can be inspected using Docker:

```bash
docker run -it --rm -v /var/lib/ngfhwk8s:/mnt ubuntu:18.04 bash
ls /mnt
...
```
