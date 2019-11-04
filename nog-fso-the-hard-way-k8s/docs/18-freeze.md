# Freezing and Unfreezing Repos
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `nogfsoctl` shell:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get repos exsrv

repoId='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'

nogfsoctl get repo $repoId

workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} freeze --workflow=$workflow --author='A U Thor <author@example.org>'
```

With the path from:

```bash
nogfsoctl get repo $repoId | jq -r .file | cut -d : -f 2
```

In a second terminal in the `storage-shell`, check that the immutable attribute
has been set:

```bash
kubectl attach -it statefulset.apps/storage

lsattr /srv/exorg_exsrv/...
```

In the `nogfsoctl` shell, unfreeze the repo:

```bash
workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} unfreeze --workflow=$workflow --author='A U Thor <author@example.org>'
```

In the `storage-shell`, check that the immutable attribute has been cleared:

```bash
lsattr /srv/exorg_exsrv/...
```
