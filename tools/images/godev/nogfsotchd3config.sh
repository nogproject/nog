# `cfgStatAuthor` is the author for commits that result from mtime range stat
# checks.
cfgStatAuthor='fso touch daemon <admin@example.com>'

# `cfgNogfsoschdTouch` defines the `nogfsoschd` arguments that control when to
# perform mtime range checks on repos.  Here:
#
#  - process repos with prefix `/example/orgfs2` in registry `exreg`;
#  - scan during start, `--scan-start`, and every hour, `--scan-every=1h`;
#  - do not observe ref updates, `--no-watch`.
#
cfgNogfsoschdTouch() {
    nogfsoschd \
        --log=mu \
        --tls-cert=/nog/ssl/certs/nogfsotchd3/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsotchd3/ca.pem \
        --sys-jwt="/nog/jwt/tokens/nogfsotchd3-registry.jwt" \
        --host='files.example.com' \
        --registry=exreg \
        --prefix=/example/orgfs2 \
        --no-watch \
        --scan-start \
        --scan-every=1h \
        "$@"
}

# `cfgNogfsoctlJwtAuth` defines the `nogfsoctl` arguments for commands that
# need to pass tokens to `nogfsostad`, which requires contacting `nog-app` to
# exchange the token for a short-term token, as indicated by the suffix
# `JwtAuth`.  `nogfsoregd` only allows forwarding short-term tokens to
# `nogfsostad`.
cfgNogfsoctlJwtAuth() {
    nogfsoctl \
        --tls-cert=/nog/ssl/certs/nogfsotchd3/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsotchd3/ca.pem \
        --jwt="/nog/jwt/tokens/nogfsotchd3-nogapp.jwt" \
        "$@"
}
