# Initializing the FSO Registries
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On `ops.example.org`:

Initialize the registries:

```bash
nogfsoctl get registries

nogfsoctl init registry --no-vid exsrv
nogfsoctl init registry --no-vid exorg

nogfsoctl get registries
nogfsoctl events registry exsrv
nogfsoctl events registry exorg
```
