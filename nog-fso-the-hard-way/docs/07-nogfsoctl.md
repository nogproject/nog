# Bootstrapping the Admin Command Line Tool
By Steffen Prohaska
<!--@@VERSIONINC@@-->

On host `ops.example.org`:

Install useful tools:

```bash
apt-get install -y python-pip uuid-runtime
pip install yq
```

Install `nogfsoctl`:

```bash
mkdir /tmp/fso
tar -C /tmp/fso -xvf /host/local/release/nogfso.tar.bz2
install -m 0755 /tmp/fso/bin/nogfsoctl /usr/local/bin/nogfsoctl
rm -rf /tmp/fso
```

Install a client TLS cert:

```bash
install -m 0700 -d ~/.nogfso
install -m 0644 /host/local/pki/tls/ca.pem ~/.nogfso/ca.pem
install -m 0600 /host/local/pki/tls/alice-combined.pem ~/.nogfso/cert-combined.pem
```

Open <http://localhost:8080> in Chrome, and issue an admin JWT by executing the
following in the browser console:

```javascript
NogReadyJwts.callIssueToken({ path: '/sys/jwts/exo/admin/nogfsoctl-admin' }, console.log);
```

Copy the token, and install it:

```bash
NOG_JWT="eyJ..."

tr -d '"' <<<"${NOG_JWT}" | install -m 0600 /dev/stdin ~/.nogfso/jwt
unset NOG_JWT
```

Set an alias to configure `nogfsoctl`:

```bash
alias nogfsoctl="nogfsoctl --nogfsoregd=fso.example.org:7550 --tls-cert=${HOME}/.nogfso/cert-combined.pem --tls-ca=${HOME}/.nogfso/ca.pem --jwt=${HOME}/.nogfso/jwt --jwt-auth=http://nog.example.org:8080/api/v1/fso/auth"

nogfsoctl get registries
```
