FROM alpine/git

RUN apk add \
      bash \
      findutils \
      rsync \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
