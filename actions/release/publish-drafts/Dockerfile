FROM alpine

RUN apk add \
    bash \
    git \
    github-cli \
    jq \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
