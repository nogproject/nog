# Bootstrapping the File Server Directory Layout
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Configure `bcpfs-perms`:

```bash
 vagrant ssh storage -- sudo install -m 0644 <<EOF /dev/stdin /etc/bcpfs.hcl
rootdir = "/srv/exorg_exsrv/data"
serviceDir = "exsrv"
orgUnitDir = "exorg"
superGroup = "ag_exorg"
orgUnitPrefix = "exorg"
servicePrefix = "exsrv"
opsSuffix = "ops"
facilitySuffix = "facility"

facility {
    name = "em"
    services = [
        "tem-505",
        "rem-707",
        "em-analysis",
    ]
    access = "perService"
}

facility {
    name = "lm"
    services = [
        "spim-100",
        "spim-222",
    ]
    access = "perService"
}

facility {
    name = "ms"
    services = [
        "ms-data",
    ]
    access = "allOrgUnits"
}

orgUnit {
    name = "ag-alice"
    subdirs = [
        { name = "people", policy = "owner" },
        { name = "service", policy = "manager" },
    ]
    extraDirs = [
        "projects",
    ]
}

# em: full ag-* list for all services
filter {
    services = [
        "tem-505",
        "rem-707",
        "em-analysis",
    ]
    orgUnit = "ag-.*"
    action = "accept"
}

# ms: reduced ag-* list for service folder
filter {
    service = "ms-data"
    orgUnits = [
        "ag-alice",
    ]
    action = "accept"
}
EOF
```

Initialize the filesystem layout:

```bash
 vagrant ssh storage -- sudo bash -sx <<\EOF
bcpfs-perms describe config
bcpfs-perms describe groups
bcpfs-perms describe org

mkdir /srv/exorg_exsrv /srv/exorg_exsrv/data
bcpfs-perms apply
EOF
```
