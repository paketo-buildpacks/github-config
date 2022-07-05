FROM alpine

RUN apk add --no-cache \
    bash \
    git \
    gnupg

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
