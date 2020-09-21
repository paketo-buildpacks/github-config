FROM alpine

RUN apk add \
    bash \
    git \
  && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
