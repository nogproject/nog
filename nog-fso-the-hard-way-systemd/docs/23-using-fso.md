# Using Nog FSO
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Until a user interface will be added to the web application, Nog FSO can be
controlled through the command line tool in the `nogfsoctl` shell:

```bash
kubectl attach -it deployments/nogfsoctl

alias nogfsoctl='nogfsoctl --nogfsoregd=fso.default.svc.cluster.local:7550 --tls-cert=/etc/nogfsoctl/cert-combined.pem --tls-ca=/etc/nogfsoctl/ca.pem --jwt=/etc/nogfsoctl/nogfsoctl.jwt --jwt-auth=http://nog.default.svc.cluster.local:8080/api/v1/fso/auth'

nogfsoctl get registries
nogfsoctl get roots exsrv
nogfsoctl get repos exsrv
```

