# Bootstrapping the Web Application
By Steffen Prohaska
<!--@@VERSIONINC@@-->

The web application is a Meteor application that has been compiled to an
application bundle that can be executed with Node.js; its dependencies are
managed with npm.  The application uses MongoDB to store state.  Users
authenticate with GitLab OAuth / OIDC.

On host `nog.example.org`:

Start MongoDB:

```bash
apt-get install -y mongodb

/etc/init.d/mongodb start
/etc/init.d/mongodb status
pgrep -fa mongod
```

Install the Meteor application bundle:

```bash
apt-get install -y nodejs npm

install -m 0755 -d /usr/local/lib/nog-app-2
tar -C /usr/local/lib/nog-app-2 -xvf /host/local/release/nog-app-2.tar.gz
( cd /usr/local/lib/nog-app-2/bundle/programs/server && npm install && npm run install )
```

Add a daemon user:

```bash
adduser --system --group nogapp
```

Install certificates:

```bash
install -m 0755 -d /usr/local/etc/nog-app-2
install -m 0644 /host/local/pki/tls/ca.pem /usr/local/etc/nog-app-2/fso-tls-cabundle.pem
install -m 0640 -g nogapp /host/local/pki/tls/nog-combined.pem /usr/local/etc/nog-app-2/fso-tls-combined.pem
install -m 0644 /host/local/pki/jwt/ca.pem /usr/local/etc/nog-app-2/fso-jwt-cabundle.pem
install -m 0640 -g nogapp /host/local/pki/jwt/nog-jwt-combined.pem /usr/local/etc/nog-app-2/fso-jwt-combined.pem
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
* Redirect URI: `http://localhost:8080/_oauth/gitimp`;
* Scopes: `read_user`, `openid`.
* Copy the Client ID and the Client secret:

```bash
gitimpClientId='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
gitimpClientSecret='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
```

Install the Meteor settings:

* `git.zib.de` OIDC is disabled by placeholders.
* The tutorial uses an example Unix domain `EXO`.  The production setup would
  use `FU`.
* The tutorial uses example registries `exsrv` and `exorg`.  The production
  setup would use specific names `bsmol` and `bcp`.

```bash
install -m 0640 -g nogapp <<EOF /dev/stdin /usr/local/etc/nog-app-2/env.sh
export ROOT_URL='http://localhost:8080'
export PORT=8080
export MONGO_URL='mongodb://localhost/nog'
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
                "addr": "fso.example.org:7550",
                "ca": "/usr/local/etc/nog-app-2/fso-tls-cabundle.pem",
                "cert": "/usr/local/etc/nog-app-2/fso-tls-combined.pem",
                "registries": ["exsrv", "exorg"],
                "domains": ["EXO"]
            }
        ],
        "jwt": {
            "ca": "/usr/local/etc/nog-app-2/fso-jwt-cabundle.pem",
            "cert": "/usr/local/etc/nog-app-2/fso-jwt-combined.pem",
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

Start the Meteor web application:

* We use `chroot` only as tool to switch the user, not to change the root
  directory.

```
chroot --userspec=nogapp / \
env HOME=/ \
bash -c 'source /usr/local/etc/nog-app-2/env.sh && cd /usr/local/lib/nog-app-2/bundle && exec node main.js'
```

Log in to <http://localhost:8080> using "Log in with ZEDAT Account".

After you have successfully logged in, the application database contains a new
user.  You can inspect it as follows:

```
$ mongo
> use nog
> db.users.findOne()
```

Configure permissions for your account:

```bash
user='<your-username>'

sed -i -e '/"permissions/,$ d' /usr/local/etc/nog-app-2/env.sh
cat <<EOF >>/usr/local/etc/nog-app-2/env.sh
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

Restart the Meteor application:

```
chroot --userspec=nogapp / \
env HOME=/ \
bash -c 'source /usr/local/etc/nog-app-2/env.sh && cd /usr/local/lib/nog-app-2/bundle && exec node main.js'
```
