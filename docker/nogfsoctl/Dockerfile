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

# <https://hub.docker.com/_/ubuntu/>, pulled 2018-07-11
FROM ubuntu:18.04@sha256:5f4bdc3467537cbbe563e80db2c3ec95d548a9145d64453b06939c4592d67b6d

LABEL maintainer="Steffen Prohaska <prohaska@zib.de>"

ENV LC_ALL=C.UTF-8

RUN set -x && \
    apt-get update && apt-get install -y \
        jq \
        python-pip \
        uuid-runtime \
    && \
    pip install yq && \
    apt-get clean && \
    :

COPY --from=build-tini /usr/local/bin/ /usr/local/bin/

ARG NOGFSOCTL_DEB
COPY ${NOGFSOCTL_DEB} /tmp/build/
RUN set -x && \
    dpkg -i "/tmp/build/${NOGFSOCTL_DEB}" && \
    rm -r /tmp/build && \
    :

ENTRYPOINT ["tini", "-g", "--"]
CMD ["bash"]
