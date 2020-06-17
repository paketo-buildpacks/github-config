FROM golang:alpine

COPY entrypoint /tmp/entrypoint
RUN cd /tmp/entrypoint && go build -o /entrypoint .

RUN apk add curl \
    && rm -rf /var/cache/apk/*

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

RUN mkdir -p "/bin" \
      && export PATH="${PATH}:/bin" \
      && curl "https://github.com/paketo-buildpacks/packit/releases/download/v0.0.14/jam-linux" \
        --silent \
        --location \
        --output "/bin/jam" \
      && chmod +x "/bin/jam"

ENTRYPOINT ["/entrypoint"]
