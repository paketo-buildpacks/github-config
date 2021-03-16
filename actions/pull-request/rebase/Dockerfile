FROM alpine

RUN apk add \
    bash \
    curl \
    git \
  && rm -rf /var/cache/apk/*

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

ARG gh_version=1.7.0
RUN mkdir -p "/bin" \
      && export PATH="${PATH}:/bin" \
      && curl "https://github.com/cli/cli/releases/download/v${gh_version}/gh_${gh_version}_linux_amd64.tar.gz" \
        --silent \
        --location \
        --output "/tmp/gh.tgz" \
      && tar xzf "/tmp/gh.tgz" --strip-components 1 -C / \
      && chmod +x "/bin/gh"

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
