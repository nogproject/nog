# Cleanup
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Delete the Kubernetes resources:

```bash
kubectl delete statefulsets/storage
kubectl delete secrets/nogfsodomd-etc
kubectl delete secrets/nogfsorstd-etc
kubectl delete configmaps/nogfsotarsecbakd-etc
kubectl delete secrets/nogfsosdwbakd3-etc
kubectl delete secrets/nogfsotard-etc
kubectl delete secrets/nogfsostad-etc
kubectl delete persistentvolumeclaims/storage-tape
kubectl delete persistentvolumeclaims/storage-online
kubectl delete persistentvolumes/volume-1
kubectl delete persistentvolumes/volume-0

kubectl delete deployments/nogfsoctl
kubectl delete secrets/nogfsoctl-etc

kubectl delete statefulsets/nogfsoregd
kubectl delete services/fso
kubectl delete secrets/nogfsoregd-etc

kubectl delete deployments/nog-app-2
kubectl delete services/nog
kubectl delete secrets/nog-app-2-etc

kubectl delete deployments/mongo
kubectl delete secrets/mongo-etc
kubectl delete statefulsets/mongod
kubectl delete services/mongodb
kubectl delete secrets/mongod-etc
kubectl delete persistentvolumeclaims/mongodb-persistent-storage-claim-mongod-0
```

If you work with Kubernetes on Docker Desktop for Mac, remove the persistent
volume directories from the Docker VM root filesystem:

```
docker run -it --rm --cap-add LINUX_IMMUTABLE \
    -v /var/lib/ngfhwk8s:/var/lib/ngfhwk8s \
    ubuntu:18.04 bash -c '
chattr -R -i /var/lib/ngfhwk8s
rm -rf /var/lib/ngfhwk8s/volume-1
rm -rf /var/lib/ngfhwk8s/volume-0
'
```
