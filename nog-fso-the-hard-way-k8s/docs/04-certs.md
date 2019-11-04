# Generating X.509 Certificates
By Steffen Prohaska
<!--@@VERSIONINC@@-->

Nog FSO uses three separate X.509 PKIs for MongoDB TLS, for Nog FSO TLS, and
for signing Java Web Tokens (JWTs).  We use CFSSL to generate the certificates.

Configure the MongoDB PKI:

```bash
mkdir -p local/pki/mongo

cat <<EOF >local/pki/mongo/config.json
{
  "signing": {
    "default": {
      "expiry": "770h"
    },
    "profiles": {
      "client": {
        "usages": ["signing", "key encipherment", "client auth"],
        "expiry": "770h"
      },
      "server": {
        "usages": ["signing", "key encipherment", "server auth"],
        "expiry": "770h"
      },
      "clientserver": {
        "usages": ["signing", "key encipherment", "client auth", "server auth"],
        "expiry": "770h"
      }
    }
  }
}
EOF

cat <<EOF >local/pki/mongo/ca.json
{
  "CN": "Example Org MongoDB CA",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "mongo-ca" } ]
}
EOF

cat <<EOF >local/pki/mongo/mongod.json
{
  "CN": "mongod-0.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "mongo-servers" } ],
  "hosts": [
    "mongod-0.mongodb.default.svc.cluster.local"
  ]
}
EOF

cat <<EOF >local/pki/mongo/mongo.json
{
  "CN": "MongoDB shell",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "mongo-clients" } ]
}
EOF

cat <<EOF >local/pki/mongo/nog.json
{
  "CN": "nog.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "mongo-clients" } ]
}
EOF

cat <<EOF >local/pki/mongo/nogfsoregd.json
{
  "CN": "nogfsoregd.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "mongo-clients" } ]
}
EOF
```

Configure the TLS PKI:

* Some certificates are for TLS clients and some for TLS servers that also act
  as TLS clients.
* The purpose of the individual certificates will become apparent over the
  course of the tutorials.

```bash
mkdir -p local/pki/tls

cat <<EOF >local/pki/tls/config.json
{
  "signing": {
    "default": {
      "expiry": "770h"
    },
    "profiles": {
      "client": {
        "usages": ["signing", "key encipherment", "client auth"],
        "expiry": "770h"
      },
      "server": {
        "usages": ["signing", "key encipherment", "server auth"],
        "expiry": "770h"
      },
      "clientserver": {
        "usages": ["signing", "key encipherment", "server auth", "client auth"],
        "expiry": "770h"
      }
    }
  }
}
EOF

cat <<EOF >local/pki/tls/ca.json
{
  "CN": "Example Org CA",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-tls-ca" } ]
}
EOF

cat <<EOF >local/pki/tls/nog.json
{
  "CN": "nog.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ]
}
EOF

cat <<EOF >local/pki/tls/nogfsoregd.json
{
  "CN": "nogfsoregd-0.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ],
  "hosts": [
    "nogfsoregd-0.fso.default.svc.cluster.local",
    "fso.default.svc.cluster.local",
    "fso.example.org"
  ]
}
EOF

cat <<EOF >local/pki/tls/alice.json
{
  "CN": "Alice Adams <aa@example.org>",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-users" } ]
}
EOF

cat <<EOF >local/pki/tls/nogfsostad.json
{
  "CN": "nogfsostad.storage.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ],
  "hosts": [ "storage.example.org" ]
}
EOF

cat <<EOF >local/pki/tls/nogfsotard.json
{
  "CN": "nogfsotard.storage.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ]
}
EOF

cat <<EOF >local/pki/tls/nogfsosdwbakd3.json
{
  "CN": "nogfsosdwbakd3.storage.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ]
}
EOF

cat <<EOF >local/pki/tls/nogfsorstd.json
{
  "CN": "nogfsorstd.storage.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ]
}
EOF

cat <<EOF >local/pki/tls/nogfsodomd.json
{
  "CN": "nogfsodomd.storage.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-servers" } ]
}
EOF
```

Configure the JWT PKI:

* The CA signs the signing key and the signing key will be used to sign JWTs
  using algorithm RS256.

```bash
mkdir -p local/pki/jwt

cat <<EOF >local/pki/jwt/config.json
{
  "signing": {
    "default": {
      "expiry": "770h"
    },
    "profiles": {
      "signing": {
         "usages": ["signing"],
         "expiry": "770h"
      }
    }
  }
}
EOF

cat <<EOF >local/pki/jwt/ca.json
{
  "CN": "Example Org CA",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-jwt-ca" } ]
}
EOF

cat <<EOF >local/pki/jwt/nog-jwt.json
{
  "CN": "nog.example.org",
  "key": { "algo": "rsa", "size": 2048 },
  "names": [ { "C": "DE", "O": "EXO", "OU": "nog-jwt" }]
}
EOF
```

Run CFSSL to generate certificates:

Unless you have `cfssl` and `cfssljson` installed, use a container:

```bash
docker run -it --rm -v $PWD/local:/host/local golang:1.12.6
go get github.com/cloudflare/cfssl/cmd/{cfssl,cfssljson}
cd /host
```

Generate the MongoDB CA and certificates:

```bash
pushd local/pki/mongo

cfssl gencert -initca ca.json \
| cfssljson -bare ca

cfssl gencert -profile=clientserver -ca=ca.pem -ca-key=ca-key.pem -config=config.json mongod.json \
| cfssljson -bare mongod
cat mongod.pem mongod-key.pem >mongod-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json mongo.json \
| cfssljson -bare mongo
cat mongo.pem mongo-key.pem >mongo-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nog.json \
| cfssljson -bare nog
cat nog.pem nog-key.pem >nog-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsoregd.json \
| cfssljson -bare nogfsoregd
cat nogfsoregd.pem nogfsoregd-key.pem >nogfsoregd-combined.pem

popd
```

Generate the TLS CA and certificates:

```bash
pushd local/pki/tls

cfssl gencert -initca ca.json \
| cfssljson -bare ca

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nog.json \
| cfssljson -bare nog
cat nog.pem nog-key.pem >nog-combined.pem

cfssl gencert -profile=clientserver -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsoregd.json \
| cfssljson -bare nogfsoregd
cat nogfsoregd.pem nogfsoregd-key.pem >nogfsoregd-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json alice.json \
| cfssljson -bare alice
cat alice.pem alice-key.pem >alice-combined.pem

cfssl gencert -profile=clientserver -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsostad.json \
| cfssljson -bare nogfsostad
cat nogfsostad.pem nogfsostad-key.pem >nogfsostad-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsotard.json \
| cfssljson -bare nogfsotard
cat nogfsotard.pem nogfsotard-key.pem >nogfsotard-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsosdwbakd3.json \
| cfssljson -bare nogfsosdwbakd3
cat nogfsosdwbakd3.pem nogfsosdwbakd3-key.pem >nogfsosdwbakd3-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsorstd.json \
| cfssljson -bare nogfsorstd
cat nogfsorstd.pem nogfsorstd-key.pem >nogfsorstd-combined.pem

cfssl gencert -profile=client -ca=ca.pem -ca-key=ca-key.pem -config=config.json nogfsodomd.json \
| cfssljson -bare nogfsodomd
cat nogfsodomd.pem nogfsodomd-key.pem >nogfsodomd-combined.pem

popd
```

Generate the JWT CA and signing key:

```bash
pushd local/pki/jwt

cfssl gencert -initca ca.json \
| cfssljson -bare ca

cfssl gencert -profile=signing -ca=ca.pem -ca-key=ca-key.pem -config=config.json nog-jwt.json \
| cfssljson -bare nog-jwt
cat nog-jwt.pem nog-jwt-key.pem >nog-jwt-combined.pem

popd
```
