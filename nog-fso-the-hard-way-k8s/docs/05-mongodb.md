# Bootstrapping MongoDB
By Steffen Prohaska
<!--@@VERSIONINC@@-->

The idea how to deploy MongoDB on Kubernetes is based on
<https://github.com/pkdone/minikube-mongodb-demo>.

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/mongod-etc
cp local/pki/mongo/ca.pem local/k8s/secrets/mongod-etc/mongo-cabundle.pem
cp local/pki/mongo/mongod-combined.pem local/k8s/secrets/mongod-etc/mongo-combined.pem
```

Create the secret:

```bash
(
    cd local/k8s/secrets/mongod-etc && \
    kubectl create secret generic \
        mongod-etc $(printf -- '--from-file=%s ' *.pem) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Create a Kubernetes service and a stateful set with MongoDB without SSL for
initialization:

```
 kubectl apply -f - <<\EOF
apiVersion: v1
kind: Service
metadata:
  name: mongodb
  labels:
    app: mongodb
spec:
  selector:
    app: mongodb
  ports:
    - port: 27017
      targetPort: 27017
      name: mongodb
  clusterIP: None
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mongod
spec:
  serviceName: mongodb
  replicas: 1
  selector:
    matchLabels:
      app: mongodb
  template:
    metadata:
      labels:
        app: mongodb
    spec:
      volumes:
        - name: mongod-etc
          secret:
            secretName: mongod-etc
            defaultMode: 0444
      containers:
        - name: mongod
          image: mongo
          ports:
            - containerPort: 27017
          volumeMounts:
            - name: mongodb-persistent-storage-claim
              mountPath: /data/db
            - name: mongod-etc
              mountPath: /usr/local/etc/mongod
              readOnly: true
          command:
            - "mongod"
            - "--wiredTigerCacheSizeGB"
            - "0.1"
            - "--bind_ip"
            - "0.0.0.0"
            - "--replSet"
            - "rs0"
  volumeClaimTemplates:
    - metadata:
        name: mongodb-persistent-storage-claim
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
EOF
```

Wait until the pod gets healthy:

```bash
kubectl get pod mongod-0
kubectl describe pod mongod-0
kubectl logs mongod-0
kubectl describe services/mongodb
```

Initialize the replica set:

```
kubectl exec mongod-0 -c mongod -- mongo --eval '
    rs.initiate({
        _id: "rs0",
        version: 1,
        members: [ {_id: 0, host: "mongod-0.mongodb.default.svc.cluster.local:27017"}
    ]});
'
```

Enable MongoDB SSL:

```
 kubectl apply -f - <<\EOF
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mongod
spec:
  serviceName: mongodb
  replicas: 1
  selector:
    matchLabels:
      app: mongodb
  template:
    metadata:
      labels:
        app: mongodb
    spec:
      volumes:
        - name: mongod-etc
          secret:
            secretName: mongod-etc
            defaultMode: 0444
      containers:
        - name: mongod
          image: mongo
          ports:
            - containerPort: 27017
          volumeMounts:
            - name: mongodb-persistent-storage-claim
              mountPath: /data/db
            - name: mongod-etc
              mountPath: /usr/local/etc/mongod
              readOnly: true
          command:
            - "mongod"
            - "--wiredTigerCacheSizeGB"
            - "0.1"
            - "--bind_ip"
            - "0.0.0.0"
            - '--sslMode'
            - 'requireSSL'
            - '--sslCAFile'
            - '/usr/local/etc/mongod/mongo-cabundle.pem'
            - '--sslPEMKeyFile'
            - '/usr/local/etc/mongod/mongo-combined.pem'
            - "--replSet"
            - "rs0"
  volumeClaimTemplates:
    - metadata:
        name: mongodb-persistent-storage-claim
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
EOF
```

MongoDB is available from inside the Kubernetes cluster at the service address:

```
mongod-0.mongodb.default.svc.cluster.local:27017
```

To use a mongo shell:

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/mongo-etc
cp local/pki/mongo/ca.pem local/k8s/secrets/mongo-etc/mongo-cabundle.pem
cp local/pki/mongo/mongo-combined.pem local/k8s/secrets/mongo-etc/mongo-combined.pem
```

Create the secret:

```bash
(
    cd local/k8s/secrets/mongo-etc && \
    kubectl create secret generic \
        mongo-etc $(printf -- '--from-file=%s ' *.pem) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Deploy the MongoDB shell:

```
 kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mongo
  namespace: default
  labels:
    app: mongo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mongo
  template:
    metadata:
      labels:
        app: mongo
    spec:
      volumes:
        - name: mongo-etc
          secret:
            secretName: mongo-etc
            defaultMode: 0444
      containers:
        - name: mongo
          image: mongo
          stdin: true
          tty: true
          volumeMounts:
            - name: mongo-etc
              mountPath: /usr/local/etc/mongo
              readOnly: true
          command:
            - 'mongo'
            - '--ssl'
            - '--sslCAFile'
            - '/usr/local/etc/mongo/mongo-cabundle.pem'
            - '--sslPEMKeyFile'
            - '/usr/local/etc/mongo/mongo-combined.pem'
            - 'mongod-0.mongodb.default.svc.cluster.local:27017'
EOF
```

Attach the mongo shell:

```
kubectl attach -it deployments/mongo
```

To detach, type `<C-P><C-Q>`.
