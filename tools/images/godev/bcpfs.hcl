# Based on
# `fuimages_bcpfs_2017/bcpfs/cmd/bcpfs-perms/generic-example-bcpfs.hcl`.  See
# there for detailed comments.

rootdir = "/orgfs2/data"
serviceDir = "srv"
orgUnitDir = "org"
superGroup = "ag_org"
orgUnitPrefix = "org"
servicePrefix = "srv"
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

facility {
    name = "fake"
    services = [
        "fake-analysis",
        "fake-tem",
    ]
    access = "perService"
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

# Reject directories `/orgfs/data/srv/*/nog` and symlinks
# `/orgfs/data/org/nog/*`.  Accept combinations that have 'fake' in both
# `service` and `orgUnit` Reject combinations that have 'fake' only in one
# component.  Accept `orgUnits` that start with `ag-`.  Reject all other
# `orgUnits`.
filter {
    service = ".*"
    orgUnit = "nog"
    action = "reject"
}

filter {
    service = "fake.*"
    orgUnit = ".*fake.*"
    action = "accept"
}

filter {
    service = "fake.*"
    orgUnit = ".*"
    action = "reject"
}

filter {
    service = ".*"
    orgUnit = ".*fake.*"
    action = "reject"
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

# Add explicit symlinks, similar to the production setup.
symlink {
    target = "../../fake-facility/service/guides"
    path = "srv/fake-tem/guides"
}

symlink {
    target = "../../fake-facility/service/guides"
    path = "srv/fake-analysis/guides"
}
