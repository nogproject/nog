# The `dev` Docker image contains a dev shell with git, make, docker, and
# docker-compose.
#
# Tini ensures quick exit on CTRL-C for any command; see
# <https://github.com/krallin/tini>.

# Base image <https://hub.docker.com/_/ubuntu/>.
FROM ubuntu:18.04@sha256:c303f19cfe9ee92badbbbd7567bc1ca47789f79303ddcef56f77687d4744cd7a

LABEL maintainer "Steffen Prohaska <prohaska@zib.de>"

# ISO C standard locale, but with UTF-8.
ENV LC_ALL=C.UTF-8

# Tini <https://github.com/krallin/tini/releases>
# Docker <https://download.docker.com/linux/static/stable/x86_64>
# Docker Compose <https://pypi.org/project/docker-compose/>
ENV \
    TINI_VERSION=v0.18.0 \
    DOCKER_VERSION=19.03.1 \
    DOCKER_SHA256=6e7d8e24ee46b13d7547d751696d01607d19c8224c1b2c867acc8c779e77734b \
    DOCKER_COMPOSE_VERSION=1.24.1 \
    K8S_VERSION=v1.15.2 \
    KUBECTL_SHA256=a737000af13f9c8c347945bc581b819659c464eae92056424bdddae735e2e888

RUN set -x && \
    apt-get update && \
    apt-get install -y \
        curl \
        git \
        jq \
        make \
        python3-pip \
    && \
    apt-get clean && \
    pip3 install \
        yq \
    && \
    :

# Tini
RUN set -x && \
    cd /tmp && \
    curl -sSLO https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini && \
    curl -sSLO https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini.asc && \
    gpg --no-tty --keyserver keyserver.ubuntu.com --recv-keys 6380DC428747F6C393FEACA59A84159D7001A4E5 && \
    gpg --no-tty --verify tini.asc && \
    install tini /usr/local/bin/tini && \
    rm tini.asc tini && \
    :

# Docker CLI
RUN set -x && \
    cd /tmp && \
    curl -sSLO https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz && \
    echo "${DOCKER_SHA256}  docker-${DOCKER_VERSION}.tgz" | sha256sum -c && \
    tar -xvf docker-${DOCKER_VERSION}.tgz docker/docker && \
    install docker/docker /usr/local/bin/docker && \
    rm -r docker docker-${DOCKER_VERSION}.tgz && \
    :

# Docker Compose
RUN set -x && \
    pip3 install docker-compose==${DOCKER_COMPOSE_VERSION} && \
    :

# Kubectl
RUN set -x && \
    cd /tmp && \
    curl -sSLO https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl && \
    echo "${KUBECTL_SHA256}  kubectl" | sha256sum -c && \
    install kubectl /usr/local/bin/kubectl && \
    rm kubectl && \
    :

# Prekustomize dependencies
RUN set -x && \
    pip3 install \
        docopt \
        jinja2 \
        jsonschema \
        pyyaml \
    ;

# Tini with signal propagation to child group to ensure quick exit for any
# command.
ENTRYPOINT ["tini", "-g", "--"]
CMD ["bash"]
