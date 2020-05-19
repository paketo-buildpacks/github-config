FROM alpine/git

RUN apk add curl \
    rsync \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
