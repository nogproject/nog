# The official Node image is at <https://hub.docker.com/r/library/node/>.  The
# build stage uses the default image, so that `npm install` has access to the
# full build toolchain.  The final image uses the slim variant to run the
# application.
#
# `tini -g --` ensures correct signal handling; see
# <https://github.com/krallin/tini>.  Tini could be dropped here if `docker
# --init` was used.  Docker Compose, however, did not support `docker --init`
# in the past; see <https://github.com/docker/compose/issues/4415>,
# <https://github.com/docker/compose/pull/4716>.  So we bake Tini in the image
# for backward compatibility.

# <https://hub.docker.com/_/buildpack-deps/>, pulled 2019-04-17
FROM buildpack-deps:bionic-curl@sha256:83d9a11ae6f93974c20fd5f320bbce96315ce0fb58a62d42666e8bd798c3d931 as build-tini

# <https://github.com/krallin/tini/releases>
ENV TINI_VERSION=v0.18.0

# The key has been exported with:
# gpg --armor --export-options export-minimal --export 6380DC428747F6C393FEACA59A84159D7001A4E5 >docker/nog-app-2/6380DC428747F6C393FEACA59A84159D7001A4E5.asc
COPY 6380DC428747F6C393FEACA59A84159D7001A4E5.asc /tmp/build/
RUN set -x && \
    gpg --import /tmp/build/6380DC428747F6C393FEACA59A84159D7001A4E5.asc && \
    echo 'trust-model tofu' | install -m 0600 /dev/stdin ~/.gnupg/gpg.conf && \
    gpg --tofu-policy good 6380DC428747F6C393FEACA59A84159D7001A4E5 && \
    curl -sSL https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini -o /tmp/tini && \
    curl -sSL https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini.asc -o /tmp/tini.asc && \
    gpg --verify /tmp/tini.asc && \
    install -m 0755 /tmp/tini /usr/local/bin/tini && \
    rm -r /tmp/tini.asc /tmp/tini /tmp/build && \
    :

# <https://hub.docker.com/r/library/node/>, pulled 2019-04-17.
FROM node:8.11.4@sha256:3422df4f7532b26b55275ad7b6dc17ec35f77192b04ce22e62e43541f3d28eb3 as build-meteor

ARG NOG_APP_2_DEB

COPY ${NOG_APP_2_DEB} /tmp/build/
RUN set -x && \
    dpkg -i --ignore-depends=nodejs,npm "/tmp/build/${NOG_APP_2_DEB}" && \
    rm -r /tmp/build && \
    :

# <https://hub.docker.com/r/library/node/>, pulled 2019-04-17.
FROM node:8.11.4-slim@sha256:682383b9e173828b786e3d3513739e9280492d3ea249655b03753dfc3bd0111d

LABEL maintainer="Steffen Prohaska <prohaska@zib.de>"

ENV LC_ALL=C.UTF-8

COPY --from=build-tini /usr/local/bin/ /usr/local/bin/
COPY --from=build-meteor /usr/lib/nog-app-2/ /usr/lib/nog-app-2/
COPY --from=build-meteor /usr/bin/nogapp2 /usr/bin/nogapp2

RUN set -x && \
    adduser --system --group --home /nonexistent --no-create-home nogapp && \
    :

# Configuration should be mounted at `/etc/nog-app-2`.
VOLUME /etc/nog-app-2

ENTRYPOINT ["tini", "-g", "--"]
CMD ["nogapp2"]
