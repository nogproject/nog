# Freezing and Unfreezing Repos
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `ops.example.org`, freeze a repo:

```bash
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

On `storage.example.org`, check that the immutable attribute has been set:

```bash
lsattr /srv/exorg_exsrv/...
```

On `ops.example.org`, unfreeze the repo:

```bash
workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} unfreeze --workflow=$workflow --author='A U Thor <author@example.org>'
```

On `storage.example.org`, check that the immutable attribute has been cleared:

```bash
lsattr /srv/exorg_exsrv/...
```
