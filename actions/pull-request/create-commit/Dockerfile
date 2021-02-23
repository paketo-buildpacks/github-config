FROM alpine

RUN apk add \
    bash \
    git \
    gnupg \
  && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
