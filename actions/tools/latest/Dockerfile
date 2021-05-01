FROM alpine/git

RUN apk add \
      bash \
      curl \
      jq \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
