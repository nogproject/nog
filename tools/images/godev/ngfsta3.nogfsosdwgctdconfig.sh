# `cfgShadowHost` is the shadow hostname that repos are expected to use.
cfgShadowHost='files.example.com'
# `cfgShadowRoot` is the shadow path prefix that repos are expected to use.
cfgShadowRoot='/nogfso/shadow'

# `cfgNogfsoschdShadowGc` defines the `nogfsoschd` command and arguments that
# control when to perform garbage collection.  Here:
#
#  - process repos below prefixes of org unit `ag-charly`;
#  - regular scans, `--scan-start` and `--scan-every`;
#  - without monitoring ref updates, `--no-watch`.
#
cfgNogfsoschdShadowGc() {
    nogfsoschd \
        --log=mu \
        --tls-cert=/nog/ssl/certs/nogfsosdwgctd/combined.pem \
        --tls-ca=/nog/ssl/certs/nogfsosdwgctd/ca.pem \
        --sys-jwt="/nog/jwt/tokens/nogfsosdwgctd.jwt" \
        --host="${cfgShadowHost}" \
        --registry=exreg \
        --prefix=/example/orgfs2/srv/tem-505/ag-charly \
        --no-watch \
        --scan-start \
        --scan-every=1h \
        "$@"
}
