FROM alpine/git

RUN apk add \
      bash \
      curl \
      curl-dev \
      jq \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
