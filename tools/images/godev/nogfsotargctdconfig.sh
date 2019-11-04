# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='files.example.com'

# `cfgNogfsoschdTarttGc` defines the `nogfsoschd` command and arguments that
# control when `nogfsotargctd` performs garbage collection on a tartt repo.
# Here:
#
#  - regular scans without watching ref updates.
#
cfgNogfsoschdTarttGc() {
    nogfsoschd \
        --log=mu \
        --tls-cert=/nog/ssl/certs/nogfsotard/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsotard/ca.pem \
        --sys-jwt="/nog/jwt/tokens/nogfsotard.jwt" \
        --host="${cfgShadowHost}" \
        --registry=exreg \
        --prefix=/example/orgfs2 \
        --no-watch \
        --scan-start \
        --scan-every=1h \
        "$@"
}
