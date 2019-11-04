FROM alpine:3.6@sha256:1072e499f3f655a032e88542330cf75b02e7bdf673278f701d7ba61629ee3ebe
LABEL maintainer="Steffen Prohaska <prohaska@zib.de>"

ENV AWSCLI_VERSION=1.11.115 \
    AWS_ACCESS_KEY_ID=Cdemo \
    AWS_SECRET_ACCESS_KEY=Cdemosecret \
    AWS_ENDPOINT_URL=http://localhost:9000

RUN set -x && \
    apk add --no-cache \
        curl \
        python3 \
        tini \
    && \
    pip3 install awscli==${AWSCLI_VERSION}

COPY miniosetup /usr/local/bin/miniosetup
RUN set -x && \
    chmod 755 /usr/local/bin/miniosetup

ENTRYPOINT ["/sbin/tini", "-g", "--"]
CMD ["/usr/local/bin/miniosetup"]
