FROM alpine

COPY entrypoint /entrypoint

RUN apk add \
    bash \
    curl \
    jq \
  && rm -rf /var/cache/apk/*

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

ENTRYPOINT ["/entrypoint"]
