# Docker image `meteordev` is used for Meteor production builds.
#
# When changing the Dockerfile, `make clean images && make meteor`.  Also bump
# the version of the corresponding image tag in the Docker Compose file to
# force an image rebuild for all developers.  Specifically, update:
#
# ```
# image: meteordev:<project>-<date>
# ```
#
# in `../../../docker-compose.yml` to today.

# <https://hub.docker.com/_/ubuntu/>, pulled 2019-02-28
FROM ubuntu:18.04@sha256:7a47ccc3bbe8a451b500d2b53104868b46d60ee8f5b35a24b41a86077c650210

LABEL maintainer "Steffen Prohaska <prohaska@zib.de>"

# ISO C standard locale, but with UTF-8.
ENV LC_ALL=C.UTF-8

# The SHA256 is for the 1.8.0.2 `install-meteor.sh`.  It needs to be updated
# when there is a new Meteor release, even if `METEOR_VERSION` remains
# unchanged.
ENV \
    METEOR_VERSION=1.8.0.2 \
    METEOR_INSTALL_SHA256=436521bc61ff2235293944a5845d38c5146b3a79a7b547e7c9687629f276e1b1 \
    METEOR_ALLOW_SUPERUSER=true

RUN set -x && \
    apt-get update && \
    apt-get install -y \
        build-essential \
        curl \
        git \
        python \
    && \
    apt-get clean

RUN set -x && \
    curl -o /tmp/install-meteor.sh https://install.meteor.com && \
    echo "${METEOR_INSTALL_SHA256}  /tmp/install-meteor.sh" | sha256sum -c && \
    # Change the script to install the desired version.
    sed -i "s/^RELEASE=.*/RELEASE='${METEOR_VERSION}'/" /tmp/install-meteor.sh && \
    grep "^RELEASE=" /tmp/install-meteor.sh && \
    sh /tmp/install-meteor.sh && \
    rm /tmp/install-meteor.sh
