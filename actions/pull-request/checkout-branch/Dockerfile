FROM alpine

RUN apk add \
    bash \
    git \
  && rm -rf /var/cache/apk/*

RUN addgroup docker && adduser runner --ingroup docker
USER runner:docker

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
