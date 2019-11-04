# Phusion instructions:
#
# - baseimage: <https://github.com/phusion/baseimage-docker>
# - passenger: <https://github.com/phusion/passenger-docker>

# Run tests in a background container:
#
# ```bash
# docker run --security-opt=seccomp=unconfined --memory=800m -d -p 5901 -e NOG_LOAD_TEST_CONFIG="$(tr -d '\n' <<<"${NOG_LOAD_TEST_CONFIG}")" ${image} nog-load-test-accounts
# ```
#
# `seccomp=unconfined` is necessary with Docker 1.11 (and probably 1.10, too)
# to avoid the Chrome startup error:
#
# ```
# Failed to move to new namespace: PID namespaces supported, Network namespace
# supported, but failed: errno = Operation not permitted`
# ```
#
# Alternatively, start the container with `docker run -ti ... bash`, and start
# the tests manually:
#
# ```bash
# export VNC_PASSWD=<password>
# nog-load-test-accounts
# ```

FROM phusion/passenger-full:0.9.18
MAINTAINER prohaska@zib.de

ENV SELENIUM_MAJOR_MINOR=2.52 \
    SELENIUM_VERSION=2.52.0 \
    SELENIUM_SHA1=dd00de748d01f7cdeb34418b238bd3af88d3d6c6 \
    CHROMEDRIVER_VERSION=2.21 \
    CHROMEDRIVER_SHA1=7be5d5c58fa826147aa83aa61c7fb0d8ca94805b

RUN set -x \
    && useradd -m test \
    && apt-key adv \
        --keyserver keyserver.ubuntu.com \
        --recv 4CCA1EAF950CEE4AB83976DCA040830F7FAC5991 \
    && echo 'deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main' \
        > /etc/apt/sources.list.d/google.list \
    && apt-get update \
    && apt-get -y install \
        fluxbox \
        google-chrome-stable \
        unzip \
        x11vnc \
        xvfb \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN set -x \
    && curl -fSL \
        "http://selenium-release.storage.googleapis.com/${SELENIUM_MAJOR_MINOR}/selenium-server-standalone-${SELENIUM_VERSION}.jar" \
        -o /usr/bin/selenium.jar \
    && echo "${SELENIUM_SHA1}  /usr/bin/selenium.jar" | sha1sum -c - \
    \
    && echo '#!/bin/bash' >/usr/bin/selenium \
    && echo >>/usr/bin/selenium \
    && echo 'exec java -jar /usr/bin/selenium.jar "$@"' >>/usr/bin/selenium \
    \
    && chmod a+x /usr/bin/selenium

RUN set -x \
    && curl -fSL \
        "http://chromedriver.storage.googleapis.com/${CHROMEDRIVER_VERSION}/chromedriver_linux64.zip" \
        -o /usr/bin/chromedriver.zip \
    && echo "${CHROMEDRIVER_SHA1}  /usr/bin/chromedriver.zip" | sha1sum -c - \
    && cd /usr/bin \
    && unzip chromedriver.zip \
    && chmod 755 chromedriver \
    && rm chromedriver.zip

RUN set -x \
    && npm install -g nightwatch

COPY src /opt/nog-load-test/
COPY nog-load-test-accounts /usr/bin/nog-load-test-accounts

RUN set -x \
    && chmod 755 /usr/bin/nog-load-test-accounts \
    && cd /opt/nog-load-test/ \
    && npm install \
    && chown -R test:test /opt/nog-load-test/

EXPOSE 5901
