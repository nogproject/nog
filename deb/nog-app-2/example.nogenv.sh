# These are example settings for illustration.  They will not immediately work.
# At least, MongoDB must be configured and X.509 certificates be installed.

# `NOG_USER` is the Unix user as which `nogapp2` will exec the Meteor
# application bundle.
export NOG_USER='nogapp'

# Meteor environment variables, see
# <https://docs.meteor.com/environment-variables.html>.
export ROOT_URL='http://localhost:8080'
export PORT=8080
export MONGO_URL='mongodb://localhost/nog'
# export MONGO_OPLOG_URL='mongodb://...'
export METEOR_SETTINGS='
{
    "GITIMP_CLIENT_ID": "0000000000000000000000000000000000000000000000000000000000000000",
    "GITIMP_CLIENT_SECRET": "0000000000000000000000000000000000000000000000000000000000000000",
    "GITZIB_CLIENT_ID": "0000000000000000000000000000000000000000000000000000000000000000",
    "GITZIB_CLIENT_SECRET": "0000000000000000000000000000000000000000000000000000000000000000",
    "oauthSecretKey": "EXAMPLExxBASE64xxxxxxx==",
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
