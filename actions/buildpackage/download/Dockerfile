FROM alpine

RUN apk add \
      jq \
      curl \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
