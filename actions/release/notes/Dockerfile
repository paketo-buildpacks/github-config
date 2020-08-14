FROM alpine

COPY entrypoint /entrypoint

RUN apk add \
    bash \
    curl \
  && rm -rf /var/cache/apk/*

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

ARG jam_version=v0.2.6
RUN mkdir -p "/bin" \
      && export PATH="${PATH}:/bin" \
      && curl "https://github.com/paketo-buildpacks/packit/releases/download/${jam_version}/jam-linux" \
        --silent \
        --location \
        --output "/bin/jam" \
      && chmod +x "/bin/jam"

ENTRYPOINT ["/entrypoint"]
