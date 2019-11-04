# Bootstrapping the File Server Directory Layout
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On host `storage.example.org`:

Add example groups and users:

```bash
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
    usermod -aG "${groupAgSuper}" "${ou}-user"
done

for d in ${services}; do
    addgroup "exsrv_${d}"
done

for f in ${facilities}; do
    addgroup "exsrv_${f}-ops"
done

grep -v '^ *$' <<<"${users}" | while read -r user ou srvs; do
    adduser --system --shell /bin/bash --ingroup "exorg_${ou}" "${user}"
    usermod -aG "${groupAgSuper}" "${user}"
    for s in ${srvs}; do
        usermod -aG "exsrv_${s}" "${user}"
        echo "Added user \`${user}\` to service \`${s}\`."
    done
done
```

Install `bcpfs-perms`:

```bash
apt-get install -y acl

apt-get install -y /host/local/release/bcpfs-perms_1.2.3_amd64.deb
```

Configure `bcpfs-perms`:

```bash
install -m 0644 <<EOF /dev/stdin /usr/local/etc/bcpfs.hcl
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
alias bcpfs-perms='bcpfs-perms --config=/usr/local/etc/bcpfs.hcl'

bcpfs-perms describe config
bcpfs-perms describe groups
bcpfs-perms describe org

mkdir /srv/exorg_exsrv/data
bcpfs-perms apply
```
