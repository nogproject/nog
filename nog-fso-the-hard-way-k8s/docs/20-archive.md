# Archiving and Unarchiving Repos
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Repos at an FSO root toplevel cannot be archived.  To test archiving and
unarchiving, we need to use a subdirectory.

In the `storage-shell`, create a subdirectory:

```bash
kubectl attach -it statefulset.apps/storage

cd /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice
mkdir subdir
echo a >subdir/a.dat
```

In a second terminal in the `nogfsoctl` shell, init a repo for the
subdirectory, copy its ID, and freeze and archive it:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get repos exsrv

nogfsoctl init repo --author='A U Thor <author@example.org>' exsrv --no-vid /exsrv/rem-707/ag-alice/subdir

repoId='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'

workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} freeze --workflow=$workflow --author='A U Thor <author@example.org>'

workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} archive --workflow=$workflow --author='A U Thor <author@example.org>'
```

In the `storage-shell`, inspect the placeholder:

```bash
find /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
lsattr /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
cat /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir/README.md
```

In the `nogfsoctl` shell, unarchive to restore the original data:

```bash
workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} unarchive --workflow=$workflow --author='A U Thor <author@example.org>'
```

In the `storage-shell`, inspect that the files have been restored:

```bash
find /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
lsattr /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
```
