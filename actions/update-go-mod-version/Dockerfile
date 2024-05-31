FROM alpine

RUN apk add \
    bash \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
