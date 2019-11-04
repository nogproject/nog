FROM alpine:3.6@sha256:b09306f2dfa3c9b626006b2f1ceeeaa6fcbfac6037d18e9d0f1d407260cb0880
LABEL maintainer="Steffen Prohaska <prohaska@zib.de>"

RUN set -x && \
    apk add --no-cache \
        python3 \
        tini \
    ;

COPY /requirements.txt /requirements.txt
RUN set -x && pip3 install -r requirements.txt

COPY intermediate/nogd.py /usr/lib/python3.5/site-packages
COPY nogreplicad /usr/local/bin/nogreplicad

ENV HOME=/vault/root

ENTRYPOINT ["/sbin/tini", "-g", "--"]
CMD ["nogreplicad"]
