# Archiving and Unarchiving Repos
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Repos at an FSO root toplevel cannot be archived.  To test archiving and
unarchiving, we need to use a subdirectory.

On `storage.example.org`, create a subdirectory:

```bash
cd /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice
mkdir subdir
echo a >subdir/a.dat
```

On `ops.example.org`, init a repo for the subdirectory, copy its ID, and freeze
and archive it:

```bash
nogfsoctl init repo --author='A U Thor <author@example.org>' exsrv --no-vid /exsrv/rem-707/ag-alice/subdir

repoId='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'

workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} freeze --workflow=$workflow --author='A U Thor <author@example.org>'

workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} archive --workflow=$workflow --author='A U Thor <author@example.org>'
```

On `storage.example.org`, inspect the placeholder:

```bash
find /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
lsattr /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
cat /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir/README.md
```

On `ops.example.org`, unarchive to restore the original data:

```bash
workflow=$(uuidgen) && echo "workflow: $workflow" && \
nogfsoctl repo exsrv --no-vid ${repoId} unarchive --workflow=$workflow --author='A U Thor <author@example.org>'
```

On `storage.example.org`, inspect that the files have been restored:

```bash
find /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
lsattr /srv/exorg_exsrv/data/exsrv/rem-707/ag-alice/subdir
```
