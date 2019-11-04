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

COPY --from=build-tini /usr/local/bin/ /usr/local/bin/

# Create users before installing packages that would create default users.
RUN set -x && \
    addgroup --system ngfsta && \
    addgroup --system ngftar && \
    addgroup --system ngfbak && \
    adduser --system --home /nonexistent --no-create-home --ingroup ngfsta ngfsta && \
    adduser --system --home /nonexistent --no-create-home --ingroup ngftar ngftar && \
    adduser --system --home /nonexistent --no-create-home --ingroup ngfsta ngfrst && \
    adduser ngfrst ngftar && \
    adduser --system --home /nonexistent --no-create-home --ingroup ngfbak ngfbak && \
    addgroup --system ngfdom && \
    adduser --system --home /nonexistent --no-create-home --ingroup ngfdom ngfdom && \
    : && \
    echo 'nogfsostad nogfsostad/user string ngfsta' | debconf-set-selections && \
    echo 'nogfsostad nogfsostad/org_group string ag_exorg' | debconf-set-selections && \
    echo 'nogfsotard nogfsotard/nogfsostad_user string ngfsta' | debconf-set-selections && \
    echo 'nogfsotard nogfsotard/nogfsotard_user string ngftar' | debconf-set-selections && \
    echo 'nogfsosdwbakd3 nogfsosdwbakd3/nogfsostad_user string ngfsta' | debconf-set-selections && \
    echo 'nogfsosdwbakd3 nogfsosdwbakd3/nogfsosdwbakd3_user string ngfbak' | debconf-set-selections && \
    echo 'nogfsorstd nogfsorstd/user string ngfrst' | debconf-set-selections && \
    echo 'nogfsorstd nogfsorstd/group string ngfsta' | debconf-set-selections && \
    echo 'nogfsodomd nogfsodomd/user string ngfdom' | debconf-set-selections && \
    :

ARG TAR_INCREMENTAL_MTIME_DEB
ARG BCPFS_PERMS_DEB
ARG GIT_FSO_DEB
ARG TARTT_DEB
ARG NOGFSOCTL_DEB
ARG NOGFSOSTAD_DEB
ARG NOGFSOSCHD_DEB
ARG NOGFSOTARD_DEB
ARG NOGFSOTARSECBAKD_DEB
ARG NOGFSOSDWBAKD3_DEB
ARG NOGFSORSTD_DEB
ARG NOGFSODOMD_DEB
COPY \
        ${TAR_INCREMENTAL_MTIME_DEB} \
        ${BCPFS_PERMS_DEB} \
        ${GIT_FSO_DEB} \
        ${TARTT_DEB} \
        ${NOGFSOCTL_DEB} \
        ${NOGFSOSTAD_DEB} \
        ${NOGFSOSCHD_DEB} \
        ${NOGFSOTARD_DEB} \
        ${NOGFSOTARSECBAKD_DEB} \
        ${NOGFSOSDWBAKD3_DEB} \
        ${NOGFSORSTD_DEB} \
        ${NOGFSODOMD_DEB} \
    /tmp/build/
RUN set -x && \
    apt-get update && \
    apt-get install -y \
        "/tmp/build/${TAR_INCREMENTAL_MTIME_DEB}" \
    && \
    apt-get install -y \
        git \
        "/tmp/build/${BCPFS_PERMS_DEB}" \
        "/tmp/build/${GIT_FSO_DEB}" \
        "/tmp/build/${TARTT_DEB}" \
        "/tmp/build/${NOGFSOCTL_DEB}" \
        "/tmp/build/${NOGFSOSTAD_DEB}" \
        "/tmp/build/${NOGFSOSCHD_DEB}" \
        "/tmp/build/${NOGFSOTARD_DEB}" \
        "/tmp/build/${NOGFSOTARSECBAKD_DEB}" \
        "/tmp/build/${NOGFSOSDWBAKD3_DEB}" \
        "/tmp/build/${NOGFSORSTD_DEB}" \
        "/tmp/build/${NOGFSODOMD_DEB}" \
    && \
    rm /usr/share/git-core/templates/hooks/*.sample && \
    apt-get clean && \
    rm -r /tmp/build && \
    :

# Enable sudo for nogfsostad, nogfsotard, and nogfsosdwbakd3.
RUN set -x && \
    sed -i -e '/^#ngf.* ALL=(root)/ s/^#//' /etc/sudoers.d/nogfsostasududod && \
    : && \
    sed -i -e '/^#ngf/ s/^#//' /etc/sudoers.d/nogfsotard && \
    sed -i -e '/^# Uncomment/d' /etc/sudoers.d/nogfsotard && \
    : && \
    sed -i -e '/^#ngf/ s/^#//' /etc/sudoers.d/nogfsosdwbakd3 && \
    sed -i -e '/^# Uncomment/d' /etc/sudoers.d/nogfsosdwbakd3 && \
    :

ENTRYPOINT ["tini", "-g", "--"]
# CMD must be specified with additional args when running a container.
CMD ["bash"]
