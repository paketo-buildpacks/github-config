FROM alpine

RUN apk add \
      bash \
      jq \
      curl \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
