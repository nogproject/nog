# Initializing FSO Roots and Repos
By Steffen Prohaska
<!--@@VERSIONINC@@-->

In the `nogfsoctl` shell:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get registries
```

Initialize roots and toplevel repos:

```bash
# From storage `( cd /srv/exorg_exsrv/data/exsrv && find . -mindepth 2 -type d ) | sed -e 's,^.,/exsrv,'`
srvRoots='
/exsrv/spim-100/lm-facility
/exsrv/spim-222/lm-facility
/exsrv/ms-data/ag-alice
/exsrv/ms-data/ms-facility
/exsrv/rem-707/ag-charly
/exsrv/rem-707/ag-alice
/exsrv/rem-707/ag-bob
/exsrv/rem-707/em-facility
/exsrv/tem-505/ag-charly
/exsrv/tem-505/ag-alice
/exsrv/tem-505/ag-bob
/exsrv/tem-505/em-facility
'

for p in ${srvRoots}; do \
    nogfsoctl init root --host=storage.example.org exsrv --no-vid ${p} /srv/exorg_exsrv/data/${p}; \
done

for p in ${srvRoots}; do \
    nogfsoctl root exsrv --no-vid ${p} set-init-policy subdir-tracking-globlist 'ignore-most:*' ; \
done

for p in ${srvRoots}; do \
    nogfsoctl init repo --author='A U Thor <author@example.org>' exsrv --no-vid ${p}; \
done
```

```bash
# From storage `( cd /srv/exorg_exsrv/data/exorg && find . -mindepth 2 -type d ) | sed -e 's,^.,/exorg,'`
ouRoots='
/exorg/ag-alice/people
/exorg/ag-alice/projects
/exorg/ag-alice/service
'

for p in ${ouRoots}; do \
    nogfsoctl init root --host=storage.example.org exorg --no-vid ${p} /srv/exorg_exsrv/data/${p}; \
done

for p in ${ouRoots}; do \
    nogfsoctl root exorg --no-vid ${p} set-init-policy subdir-tracking-globlist 'ignore-most:*' ; \
done

for p in ${ouRoots}; do \
    nogfsoctl init repo --author='A U Thor <author@example.org>' exorg --no-vid ${p}; \
done
```

Inspect and modify a repo:

```bash
nogfsoctl get repos exsrv

repoId='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'

nogfsoctl get repo $repoId
nogfsoctl gitnog head $repoId
nogfsoctl ls-stat-tree $repoId $(nogfsoctl gitnog head $repoId | yq -r .statGitCommit)
nogfsoctl ls-meta-tree $repoId $(nogfsoctl gitnog head $repoId | yq -r .statGitCommit)
```

With the path from:

```bash
nogfsoctl get repo $repoId | jq -r .file | cut -d : -f 2
```

In the `storage-shell` in a second terminal, add files:

```bash
kubectl attach -it statefulset.apps/storage

cd /srv/exorg_exsrv/...
echo a >a.dat
echo b >b.dat
```

Back in the `nogfsoctl` shell, trigger a stat update of the shadow repo:

```bash
nogfsoctl stat --author='A U Thor <author@example.org>' $repoId

nogfsoctl ls-stat-tree $repoId $(nogfsoctl gitnog head $repoId | yq -r .statGitCommit)
nogfsoctl ls-meta-tree $repoId $(nogfsoctl gitnog head $repoId | yq -r .statGitCommit)
```
