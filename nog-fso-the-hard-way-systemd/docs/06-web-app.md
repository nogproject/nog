# Bootstrapping the Web Application
By Steffen Prohaska
<!--@@VERSIONINC@@-->

The web application is a Meteor application that has been compiled to an
application bundle that can be executed with Node.js; its dependencies are
managed with npm.  The application uses MongoDB to store state.  Users
authenticate with GitLab OAuth / OIDC.

We will use the image that has been set earlier:

```bash
echo "nogApp2Image: ${nogApp2Image}"
```

Create a Kubernetes secret:

Gather certificates:

```bash
mkdir -p local/k8s/secrets/nog-app-2-etc
cp local/pki/mongo/ca.pem local/k8s/secrets/nog-app-2-etc/mongo-cabundle.pem
cp local/pki/mongo/nog-combined.pem local/k8s/secrets/nog-app-2-etc/mongo-combined.pem
cp local/pki/tls/ca.pem local/k8s/secrets/nog-app-2-etc/fso-tls-cabundle.pem
cp local/pki/tls/nog-combined.pem local/k8s/secrets/nog-app-2-etc/fso-tls-combined.pem
cp local/pki/jwt/ca.pem local/k8s/secrets/nog-app-2-etc/fso-jwt-cabundle.pem
cp local/pki/jwt/nog-jwt-combined.pem local/k8s/secrets/nog-app-2-etc/fso-jwt-combined.pem
```

Prepare the Meteor settings:

Create a key that is used to encrypt OAuth tokens at rest in MongoDB:

```bash
oauthSecretKey="$(node -e 'console.log(require("crypto").randomBytes(16).toString("base64"))')"
```

Register an OAuth application in Gitlab:

* Log in to <https://git.imp.fu-berlin.de>, go to "Setting / Applications / add
  new application" with the following options:
* Name: `nog.example.org`;
* Redirect URI: `http://localhost:30080/_oauth/gitimp`;
* Scopes: `read_user`, `openid`.
* Copy the Client ID and the Client secret:

```bash
gitimpClientId='0000000000000000000000000000000000000000000000000000000000000000'
gitimpClientSecret='0000000000000000000000000000000000000000000000000000000000000000'
```

Install the Meteor settings:

* `git.zib.de` OIDC is disabled by placeholders.
* The tutorial uses an example Unix domain `EXO`.  The production setup would
  use `FU`.
* The tutorial uses example registries `exsrv` and `exorg`.  The production
  setup would use specific names `bsmol` and `bcp`.

```bash
 cat <<EOF >local/k8s/secrets/nog-app-2-etc/nogenv.sh
export NOG_USER=nogapp
export ROOT_URL='http://localhost:30080'
export PORT=8080
export MONGO_URL='mongodb://mongod-0.mongodb.default.svc.cluster.local:27017/nog'
export MONGO_SSL_CA='/etc/nog-app-2/mongo-cabundle.pem'
export MONGO_SSL_CERT='/etc/nog-app-2/mongo-combined.pem'
export MONGO_OPLOG_URL='mongodb://mongod-0.mongodb.default.svc.cluster.local:27017/local?replicaSet=rs0'
export METEOR_SETTINGS='
{
    "GITIMP_CLIENT_ID": "${gitimpClientId}",
    "GITIMP_CLIENT_SECRET": "${gitimpClientSecret}",
    "GITZIB_CLIENT_ID": "0000000000000000000000000000000000000000000000000000000000000000",
    "GITZIB_CLIENT_SECRET": "0000000000000000000000000000000000000000000000000000000000000000",
    "oauthSecretKey": "${oauthSecretKey}",
    "ldap": [],
    "fsoUnixDomains": [
      { "domain": "EXO", "service": "gitimp" }
    ],
    "wellknownAccounts": [],
    "minifso": {
        "registries": [
            {
                "name": "fso.example.org",
                "addr": "fso.default.svc.cluster.local:7550",
                "ca": "/etc/nog-app-2/fso-tls-cabundle.pem",
                "cert": "/etc/nog-app-2/fso-tls-combined.pem",
                "registries": ["exsrv", "exorg"],
                "domains": ["EXO"]
            }
        ],
        "jwt": {
            "ca": "/etc/nog-app-2/fso-jwt-cabundle.pem",
            "cert": "/etc/nog-app-2/fso-jwt-combined.pem",
            "domains": [
                { "jwtXcrd": "EXO", "service": "gitimp" }
            ],
            "issuer": "nogapp",
            "ou": "nog-jwt"
        },
        "permissions": [
        ],
        "readyJwts": [
        ]
    }
}
'
EOF
```

Create the secret:

```bash
(
    cd local/k8s/secrets/nog-app-2-etc && \
    kubectl create secret generic \
        nog-app-2-etc $(printf -- '--from-file=%s ' *.pem *.sh) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Deploy `nog-app-2`:

```
 kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: nog
  labels:
    app: nog
spec:
  selector:
    app: nog-app-2
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 8080
      name: www
      nodePort: 30080
  type: NodePort
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nog-app-2
  namespace: default
  labels:
    app: nog-app-2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nog-app-2
  template:
    metadata:
      labels:
        app: nog-app-2
    spec:
      volumes:
        - name: nog-app-2-etc
          secret:
            secretName: nog-app-2-etc
            defaultMode: 0444
      containers:
        - name: nog-app-2
          image: ${nogApp2Image}
          args: ["nogapp2", "/etc/nog-app-2/nogenv.sh"]
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: nog-app-2-etc
              mountPath: /etc/nog-app-2
              readOnly: true
EOF
```

Log in to <http://localhost:30080> using "Log in with ZEDAT Account".

After you have successfully logged in, the application database contains a new
user.  You can inspect it as follows:

```
$ kubectl attach -it deployments/mongo
rs0:PRIMARY> use nog
rs0:PRIMARY> db.users.findOne()
```

Configure permissions for your account:

```bash
user='<your-username>'

gsed -i -e '/"permissions/,$ d' local/k8s/secrets/nog-app-2-etc/nogenv.sh
 cat <<EOF >>local/k8s/secrets/nog-app-2-etc/nogenv.sh
        "permissions": [
            {
                "actions": [
                    "bc/read",
                    "fso/admin-registry",
                    "fso/exec-ping-registry",
                    "fso/init-registry",
                    "fso/read-main",
                    "fso/read-registry"
                ],
                "names": [
                    "all",
                    "allaggsig",
                    "main",
                    "exorg",
                    "exsrv"
                ],
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsNames"
            },
            {
                "actions": [
                    "fso/issue-user-token",
                    "fso/issue-sys-token",
                    "fso/read-root",
                    "fso/exec-split-root",
                    "fso/find",
                    "fso/init-root",
                    "fso/read-root",
                    "fso/delete-root",
                    "fso/enable-discovery-path",
                    "fso/admin-repo",
                    "fso/test-udo",
                    "fso/test-udo-as"
                ],
                "pathPrefix": "/",
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsPathPrefix"
            },
            {
                "actions": [
                    "bc/write"
                ],
                "names": [
                    "all"
                ],
                "principals": [
                  "username:${user}"
                ],
                "rule": "AllowPrincipalsNames"
            },
            {
                "actions": [
                    "fso/session"
                ],
                "names": [
                    "storage.example.org"
                ],
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsNames"
            },
            {
                "actions": [
                    "fso/admin-root",
                    "fso/archive-repo",
                    "fso/confirm-repo",
                    "fso/exec-archive-repo",
                    "fso/exec-du",
                    "fso/exec-freeze-repo",
                    "fso/exec-split-root",
                    "fso/exec-unarchive-repo",
                    "fso/exec-unfreeze-repo",
                    "fso/freeze-repo",
                    "fso/init-repo",
                    "fso/init-repo-shadow-backup",
                    "fso/init-repo-tartt",
                    "fso/read-repo",
                    "fso/read-root",
                    "fso/refresh-repo",
                    "fso/unarchive-repo",
                    "fso/unfreeze-repo"
                ],
                "pathPrefix": "/exsrv/",
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsPathPrefix"
            },
            {
                "actions": [
                    "fso/admin-root",
                    "fso/archive-repo",
                    "fso/confirm-repo",
                    "fso/exec-archive-repo",
                    "fso/exec-du",
                    "fso/exec-freeze-repo",
                    "fso/exec-split-root",
                    "fso/exec-unarchive-repo",
                    "fso/exec-unfreeze-repo",
                    "fso/freeze-repo",
                    "fso/init-repo",
                    "fso/init-repo-shadow-backup",
                    "fso/init-repo-tartt",
                    "fso/read-repo",
                    "fso/read-root",
                    "fso/refresh-repo",
                    "fso/unarchive-repo",
                    "fso/unfreeze-repo"
                ],
                "pathPrefix": "/exorg/",
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsPathPrefix"
            },
            {
                "actions": [
                    "uxd/init-unix-domain",
                    "uxd/read-unix-domain",
                    "uxd/write-unix-domain"
                ],
                "names": [
                    "EXO"
                ],
                "principals": [
                    "username:${user}"
                ],
                "rule": "AllowPrincipalsNames"
            },
            {
              "actions": [
                  "fso/issue-ready-jwt"
              ],
              "pathPrefix": "/sys/jwts/exo/admin/",
              "principals": [
                  "username:${user}"
              ],
              "rule": "AllowPrincipalsPathPrefix"
            }
        ],
        "readyJwts": [
            {
                "description": "nogfsoctl-admin token allows admins to run nogfsoctl on /exsrv and /exorg",
                "expiresIn": 5443200,
                "path": "/sys/jwts/exo/admin/nogfsoctl-admin",
                "scopes": [
                    {
                        "action": "api",
                        "path": "/auth"
                    },
                    {
                        "action": "fso/issue-user-token",
                        "path": "/"
                    },
                    {
                        "action": "bc/*",
                        "name": "all"
                    },
                    {
                        "action": "fso/*",
                        "names": [
                            "main",
                            "exsrv",
                            "exorg"
                        ],
                        "paths": [
                            "/exsrv/*",
                            "/exorg/*"
                        ]
                    },
                    {
                        "action": "uxd/*",
                        "names": [
                            "EXO"
                        ]
                    }
                ],
                "subuser": "nogfsoctl-admin",
                "title": "nogfsoctl-admin"
            },
            {
                "description": "issue-jwts-regd is a short term token to issue nogfsoregd JWTs",
                "expiresIn": 3660,
                "path": "/sys/jwts/exo/admin/issue-jwts-regd",
                "scopes": [
                    {
                        "action": "api",
                        "path": "/auth"
                    },
                    {
                        "action": "fso/issue-sys-token",
                        "path": "/"
                    },
                    {
                        "action": "bc/read",
                        "name": "allaggsig"
                    },
                    {
                        "actions": [
                            "fso/read-registry",
                            "fso/exec-ping-registry"
                        ],
                        "names": [
                            "exsrv",
                            "exorg"
                        ]
                    },
                    {
                        "actions": [
                            "fso/read-root",
                            "fso/read-repo",
                            "fso/exec-split-root",
                            "fso/exec-freeze-repo",
                            "fso/exec-unfreeze-repo",
                            "fso/exec-archive-repo",
                            "fso/exec-unarchive-repo"
                        ],
                        "paths": [
                            "/exsrv/*",
                            "/exorg/*"
                        ]
                    }
                ],
                "subuser": "issue-jwts-regd",
                "title": "issue-jwts-regd"
            },
            {
                "description": "issue-jwts-stad is a short term token to issue nogfsostad JWTs",
                "expiresIn": 3660,
                "path": "/sys/jwts/exo/admin/issue-jwts-stad",
                "scopes": [
                    {
                        "action": "api",
                        "path": "/auth"
                    },
                    {
                        "action": "fso/issue-sys-token",
                        "path": "/"
                    },
                    {
                        "actions": [
                            "bc/read",
                            "bc/write"
                        ],
                        "names": [
                            "all",
                            "allaggsig"
                        ]
                    },
                    {
                        "action": "fso/session",
                        "name": "storage.example.org"
                    },
                    {
                        "actions": [
                            "fso/read-registry",
                            "fso/exec-ping-registry"
                        ],
                        "names": [
                            "exsrv",
                            "exorg"
                        ]
                    },
                    {
                        "actions": [
                            "fso/read-root",
                            "fso/exec-du",
                            "fso/exec-split-root",
                            "fso/read-repo",
                            "fso/confirm-repo",
                            "fso/init-repo-shadow-backup",
                            "fso/init-repo-tartt",
                            "fso/exec-freeze-repo",
                            "fso/exec-unfreeze-repo",
                            "fso/exec-archive-repo",
                            "fso/exec-unarchive-repo"
                        ],
                        "paths": [
                            "/exsrv/*",
                            "/exorg/*"
                        ]
                    },
                    {
                        "actions": [
                            "uxd/read-unix-domain",
                            "uxd/write-unix-domain"
                        ],
                        "names": [
                            "EXO"
                        ]
                    }
                ],
                "subuser": "issue-jwts-stad",
                "title": "issue-jwts-stad"
            }
        ]
    }
}
'
EOF
```

Update the secret:

```bash
(
    cd local/k8s/secrets/nog-app-2-etc && \
    kubectl create secret generic \
        nog-app-2-etc $(printf -- '--from-file=%s ' *.pem *.sh) \
        --dry-run -o yaml
) \
| kubectl apply -f -
```

Restart the Meteor application:

```
kubectl get pods | grep ^nog-app-2- | awk '{ print $1 }' | xargs kubectl delete pod
```
