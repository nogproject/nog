# The `godev` Docker image contains a Go dev environment.
#
# When changing the Dockerfile, `make clean images && make`.  Also bump the
# version of the corresponding image tag in the Docker Compose file to force an
# image rebuild for all developers.  Specifically, update `image:
# godev:<project>-<date>` in `../../../docker-compose.yml` to today.
#
# The Dockerfile installs GRPC similar to the `grpc/go` image, see
# <https://hub.docker.com/r/grpc/go/~/dockerfile/>,
# <https://github.com/grpc/grpc-docker-library/blob/master/1.0/golang/Dockerfile>.
#
# Tini ensures quick exit on CTRL-C for any command; see
# <https://github.com/krallin/tini>.

FROM buildpack-deps:stretch@sha256:1b1abee8b6bc01f66238fe5ce45637861db77cb5788875165c531898ccdc751b as build-tar

ENV GNU_TAR_VERSION=5d82c6ca76c6afb9852c4cda6acf954a524c30ed

RUN set -x && \
    apt-get update && \
    apt-get install -y \
        autopoint \
        bison \
        gettext \
        rsync \
        texinfo \
    && \
    apt-get clean

RUN set -x && \
    cd /tmp && \
    git clone https://github.com/sprohaska/gnu-tar.git && \
    cd gnu-tar && \
    git checkout ${GNU_TAR_VERSION} && \
    ./bootstrap && \
    FORCE_UNSAFE_CONFIGURE=1 ./configure prefix=/usr/local && \
    make && \
    make install && \
    cd / && \
    rm -rf /tmp/gnu-tar

# Base image <https://hub.docker.com/_/golang/>.
FROM golang:1.12.6@sha256:35200a727dc44175d9221a6ece398eed7e4b8e17cb7f0d72b20bf2d5cf9dc00d

LABEL maintainer "Steffen Prohaska <prohaska@zib.de>"

# ISO C standard locale, but with UTF-8.
ENV LC_ALL=C.UTF-8

# Enable Go modules for development.
#
# The Dockerfile uses `GO111MODULE=off go get` to disable modules when install
# tools, which is one of the alternatives discussed in the Go wiki,
# <https://github.com/golang/go/wiki/Modules#why-does-installing-a-tool-via-go-get-fail-with-error-cannot-find-main-module>.
#
# To use Go modules with git2go, tell Go to use the local version with the
# following line in the repo toplevel `go.mod`:
#
# ```
# replace github.com/libgit2/git2go => /go/src/github.com/libgit2/git2go
# ```
#
# See `git2go` in one of the last build steps below.
ENV GO111MODULE=on

# <https://github.com/google/protobuf/releases>
# <https://github.com/golang/protobuf>
# <https://github.com/grpc/grpc-go/releases>
# <https://github.com/libgit2/git2go>
# <https://github.com/krallin/tini/releases>
# bcpfs-perms copied from ZIB bcpfs apt registry.
#
# When upgrading `GRPC_GO_VERSION` or `PROTOC_GEN_GO_VERSION`, ensure that the
# `go.mod` versions are updated, too:
#
# ```
# make images
# ddev bash -c 'go get google.golang.org/grpc@${GRPC_GO_VERSION}'
# ddev bash -c 'go get github.com/golang/protobuf@${PROTOC_GEN_GO_VERSION}'
# ```
#
# When upgrading `GIT2GO_VERSION`, also update the repo toplevel `go.mod` to a
# matching pseudo-version:
#
# ```
# make images
# ddev bash -c 'cd /go/src/github.com/libgit2/git2go && TZ=UTC git show -s --abbrev=12 --date=format-local:%Y%m%d%H%M%S --pretty=v0.0.0-%ad-%h'
# ```
#
ENV \
    PROTOC_SHA256=717903f32653f07cd895cfe89ca18ff4ca35f825afa7fe17bcb5cb13bf628be0 \
    PROTOC_VERSION=3.8.0 \
    PROTOC_GEN_GO_VERSION=v1.3.1 \
    GRPC_GO_VERSION=v1.22.0 \
    GIT2GO_VERSION=1381380f3450f91b66c7951ad0e40c31acb784a1 \
    TINI_VERSION=v0.18.0 \
    BCPFS_PERMS_VERSION=2.0.0 \
    BCPFS_PERMS_SHA256=791deeac705991a99664626886acfab380b94b131ae310510ad56153a09c15a3

# `cmake` for Git2go.
# `netcat-openbsd` to have `nc` for scripting.
# `uuid` to generate UUIDs for testing.
RUN set -x && \
    apt-get update && \
    apt-get -y install \
        acl \
        apt-transport-https \
        apt-utils \
        bc \
        bzip2 \
        cmake \
        gawk \
        gnupg2 \
        jq \
        libcap2-bin \
        net-tools \
        netcat-openbsd \
        python-pip \
        sudo \
        unzip \
        uuid \
        vim \
    && \
    pip install \
        yq \
    && \
    apt-get clean

# Tini
RUN set -x && \
    curl -sSL https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini -o /tmp/tini && \
    curl -sSL https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini.asc -o /tmp/tini.asc && \
    gpg --no-tty --keyserver keyserver.ubuntu.com --recv-keys 6380DC428747F6C393FEACA59A84159D7001A4E5 && \
    gpg --no-tty --verify /tmp/tini.asc && \
    chmod a+x /tmp/tini && \
    mv /tmp/tini /usr/local/bin/tini

# GRPC
RUN set -x && \
    mkdir /tmp/protoc && \
    cd /tmp/protoc && \
    curl -sSL https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip -o protoc.zip && \
    echo "${PROTOC_SHA256}  protoc.zip" | sha256sum -c && \
    unzip protoc.zip && \
    install -v -o root -g root -m 0755 bin/protoc /usr/local/bin/protoc && \
    find include -type f -exec \
        install -v -o root -g root -m 0644 -D '{}' '/usr/local/{}' ';' && \
    cd / && \
    rm -r /tmp/protoc

# GRCP Go, specific versions.
RUN set -x && \
    GO111MODULE=off go get -d \
        github.com/golang/protobuf/protoc-gen-go \
        google.golang.org/grpc \
    && \
    cd /go/src/github.com/golang/protobuf && \
    git checkout ${PROTOC_GEN_GO_VERSION} && \
    cd /go/src/google.golang.org/grpc && \
    git checkout ${GRPC_GO_VERSION} && \
    GO111MODULE=off go get \
        github.com/golang/protobuf/protoc-gen-go \
        google.golang.org/grpc \
    ;

# More Go tooling.
#
#  - errcheck: command to verify that errors are inspected.
#  - spew: pretty printing structs for debugging.
#
RUN set -x && \
    GO111MODULE=off go get \
        github.com/davecgh/go-spew/spew \
        github.com/kisielk/errcheck \
    ;

# Certs tooling.
RUN set -x && \
    GO111MODULE=off go get \
        github.com/cloudflare/cfssl/cmd/cfssl \
        github.com/cloudflare/cfssl/cmd/cfssljson \
    ;

# Git2go.
#
# The build tag `static` must be used when building dependent packages.
# Otherwise Go tries to use `pkg-config`.  See `git2go/git_dynamic.go`.
#
# Git2go is incompatible with the build tag `netgo`, because Git2go depends on
# libgit2, which uses `getaddrinfo() and, therefore, depends on glibc.
#
# Do not cleanup, because Go requires information from the build directory.
# See `git2go/git_static.go`.  Specifically, do not:
#
# ```
# git submodule deinit -- vendor && \
# rm -r .git/modules && \
# ```
#
# Patch `USE_HTTPS=OFF` to avoid shared lib dependency on OpenSSL.
#
# Run `go mod init` to initialize `git2go` as a go module, which is required by
# the `replace` statement in the repo toplevel `go.mod` file.
RUN set -x && \
    GO111MODULE=off go get -d github.com/libgit2/git2go && \
    cd /go/src/github.com/libgit2/git2go && \
    git checkout ${GIT2GO_VERSION} && \
    git submodule update --init -- vendor && \
    GO111MODULE=on go mod init github.com/libgit2/git2go && \
    sed -i -e 's/^cmake -D/cmake -DUSE_HTTPS=OFF /' script/build-libgit2-static.sh && \
    make install-static && \
    cd /

# Devcerts setup.
RUN set -x && \
    mkdir -m 0700 /root/.nogfso /root/.nogfso/certs && \
    ln -s /nog/ssl/certs/alice /root/.nogfso/certs/nogfsoctl

# Add PGP keys for testing.
RUN set -x && \
    gpg --no-tty --keyserver keyserver.ubuntu.com --recv-keys \
        57F8053FEFF71DE1AF975B3AC9DA1CE0ED3DDC51 \
        80C4AD33864BC422467D465AE938E2DA3D857F91 \
    && \
    echo 'trust-model tofu' >>/root/.gnupg/gpg.conf && \
    gpg --no-tty --tofu-policy good \
        57F8053FEFF71DE1AF975B3AC9DA1CE0ED3DDC51 \
        80C4AD33864BC422467D465AE938E2DA3D857F91 \
    && \
    gpgconf --kill gpg-agent && \
    ln -s /hostsocks/S.gpg-agent.extra /root/.gnupg/S.gpg-agent

# Simulate a bcpfs file server.
COPY \
        bcpfs-perms_${BCPFS_PERMS_VERSION}_amd64.deb \
        bcpfs.hcl \
        build/add-orgfs-groups \
    /tmp/build/
RUN : && \
    if fgrep -q 'version https://git-lfs.github.com/spec/v1' /tmp/build/bcpfs-perms_${BCPFS_PERMS_VERSION}_amd64.deb; then \
        echo >&2 "ERROR: bcpfs-perms_${BCPFS_PERMS_VERSION}_amd64.deb is a Git LFS placeholder; you need to git lfs fetch and checkout." ; \
        exit 1 ; \
    fi && \
    set -x && \
    cd /tmp/build && \
    echo "${BCPFS_PERMS_SHA256}  bcpfs-perms_${BCPFS_PERMS_VERSION}_amd64.deb" | sha256sum -c && \
    apt-get install ./bcpfs-perms_${BCPFS_PERMS_VERSION}_amd64.deb && \
    install -m 0644 bcpfs.hcl /etc/bcpfs.hcl && \
    chmod +x add-orgfs-groups && ./add-orgfs-groups && \
    rm -r /tmp/build

# Patched GNU tar with --listed-incremental-mtime.
COPY --from=build-tar /usr/local/bin/tar /usr/local/bin/tar

# Simulate a backup setup that uses capabilities to read files and sudo to push
# to shadow repos.
#
# Changing config files requires rebuilding the image, which is a bit
# inconvenient.  But we want to test the setup with `/etc/` ownership and
# permissions, which cannot be done with a bind-mount of the Git workspace.
#
# XXX `.gnupg` would better be readonly.  But it's a bit tricky, see
# for example <https://www.owlfolio.org/possibly-useful/readonly-gpg2/>.
COPY \
        nogfsotardconfig.sh \
        nogfsotargctdconfig.sh \
        nogfsotarsecbakdconfig.sh \
    /etc/nog/
COPY \
        tartt-restore-as-ngftar \
        sudoers \
    /tmp/
RUN set -x && \
    for f in \
        tartt-restore-as-ngftar \
    ; do \
        install -m 0755 /tmp/${f} /usr/local/bin/${f} && \
        rm /tmp/${f} ; \
    done && \
    : && \
    for f in \
        git-receive-pack-get-owner \
        git-receive-pack-sudo-owner \
    ; do \
        ln -s /go/src/github.com/nogproject/nog/backend/bin/${f} /usr/local/bin/${f} ; \
    done && \
    : && \
    chown root:ngftar /etc/nog/nogfsotardconfig.sh && \
    chmod 0640 /etc/nog/nogfsotardconfig.sh && \
    chown root:ngftar /etc/nog/nogfsotargctdconfig.sh && \
    chmod 0640 /etc/nog/nogfsotargctdconfig.sh && \
    chown root:ngftar /etc/nog/nogfsotarsecbakdconfig.sh && \
    chmod 0640 /etc/nog/nogfsotarsecbakdconfig.sh && \
    : && \
    install -o root -g root -m 0755 -d /usr/local/lib/nogfsotard && \
    for f in \
        git-archive-branch-dir \
        git-for-each-ref-dir \
        git-is-immutable-fso-stat-dir \
        git-is-newer-branch-dir-duration \
        git-rev-parse-branch-dir \
        git-rev-parse-is-valid-branch-dir \
        stat-dir-owner \
    ; do \
        ln -s /go/src/github.com/nogproject/nog/backend/bin/${f} /usr/local/lib/nogfsotard/${f} ; \
    done && \
    install -o root -g ngftar -m 0750 /usr/bin/git /usr/local/lib/nogfsotard/git && \
    setcap cap_dac_read_search=ep /usr/local/lib/nogfsotard/git && \
    install -o root -g ngftar -m 0750 /usr/local/bin/tar /usr/local/lib/nogfsotard/tar && \
    setcap cap_dac_read_search=ep /usr/local/lib/nogfsotard/tar && \
    : && \
    install -o root -g root -m 0755 -d /usr/local/lib/nogfsorstd && \
    install -o root -g ngftar -m 0750 /bin/tar /usr/local/lib/nogfsorstd/tar && \
    setcap cap_chown,cap_dac_override,cap_fowner=ep /usr/local/lib/nogfsorstd/tar && \
    : && \
    install -m 0755 -d /usr/local/lib/nogfsosdwbakd3 && \
    for f in \
        git-for-each-ref-dir \
        stat-dir-owner \
    ; do \
        ln -s /go/src/github.com/nogproject/nog/backend/bin/${f} /usr/local/lib/nogfsosdwbakd3/${f} ; \
    done && \
    : && \
    install -m 0440 /tmp/sudoers /etc/sudoers.d/nogfso && \
    rm /tmp/sudoers && \
    : && \
    printf '[user]\n\tname = fso archive daemon\n\temail = admin@example.com\n' \
    | install -o root -g ngftar -m 0640 /dev/stdin /home/ngftar/.gitconfig && \
    install -o ngftar -g ngftar -m 0700 -d /home/ngftar/.gnupg && \
    install -o ngftar -g ngftar -m 0400 /root/.gnupg/gpg.conf /home/ngftar/.gnupg/gpg.conf && \
    install -o ngftar -g ngftar -m 0400 /root/.gnupg/pubring.kbx /home/ngftar/.gnupg/pubring.kbx && \
    install -o ngftar -g ngftar -m 0600 /root/.gnupg/tofu.db /home/ngftar/.gnupg/tofu.db && \
    ln -s /hostsocks/S.gpg-agent.extra /home/ngftar/.gnupg/S.gpg-agent && \
    : && \
    install -m 0755 -d /usr/local/lib/tartt-restore && \
    install -o root -g ngftar -m 0750 /usr/local/bin/tar /usr/local/lib/tartt-restore/tar && \
    setcap cap_chown,cap_dac_override,cap_fowner=ep /usr/local/lib/tartt-restore/tar && \
    :

COPY \
    nogfsosdwbakd3config.sh \
    /etc/nog/
RUN set -x && \
    chown root:ngfbak /etc/nog/nogfsosdwbakd3config.sh && \
    chmod 0640 /etc/nog/nogfsosdwbakd3config.sh && \
    : && \
    install -o root -g root -m 0755 -d /usr/local/lib/nogfsosdwbakd3 && \
    install -o root -g ngfbak -m 0750 /usr/bin/git /usr/local/lib/nogfsosdwbakd3/git && \
    setcap cap_dac_read_search=ep /usr/local/lib/nogfsosdwbakd3/git && \
    install -o root -g ngfbak -m 0750 /usr/local/bin/tar /usr/local/lib/nogfsosdwbakd3/tar && \
    setcap cap_dac_read_search=ep /usr/local/lib/nogfsosdwbakd3/tar && \
    : && \
    printf '[user]\n\tname = fso backup daemon\n\temail = admin@example.com\n' \
    | install -o root -g ngfbak -m 0640 /dev/stdin /home/ngfbak/.gitconfig && \
    install -o ngfbak -g ngfbak -m 0700 -d /home/ngfbak/.gnupg && \
    install -o ngfbak -g ngfbak -m 0400 /root/.gnupg/gpg.conf /home/ngfbak/.gnupg/gpg.conf && \
    install -o ngfbak -g ngfbak -m 0400 /root/.gnupg/pubring.kbx /home/ngfbak/.gnupg/pubring.kbx && \
    install -o ngfbak -g ngfbak -m 0600 /root/.gnupg/tofu.db /home/ngfbak/.gnupg/tofu.db && \
    :

COPY \
    ngfsta2.nogfsosdwgctdconfig.sh \
    ngfsta3.nogfsosdwgctdconfig.sh \
    /etc/nog/
RUN set -x && \
    chown root:ngfsta /etc/nog/ngfsta2.nogfsosdwgctdconfig.sh && \
    chmod 0640 /etc/nog/ngfsta2.nogfsosdwgctdconfig.sh && \
    chown root:ngfsta /etc/nog/ngfsta3.nogfsosdwgctdconfig.sh && \
    chmod 0640 /etc/nog/ngfsta3.nogfsosdwgctdconfig.sh && \
    :

COPY \
    nogfsotchd3config.sh \
    /etc/nog/
RUN set -x && \
    chown root:ngftch /etc/nog/nogfsotchd3config.sh && \
    chmod 0640 /etc/nog/nogfsotchd3config.sh && \
    :

# Tini with signal propagation to child group to ensure quick exit for any
# command.
ENTRYPOINT ["tini", "-g", "--"]
