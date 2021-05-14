FROM golang:alpine

RUN apk add \
    bash \
  && rm -rf /var/cache/apk/*

COPY entrypoint /tmp/entrypoint
RUN cd /tmp/entrypoint && go build -o /entrypoint .

ENTRYPOINT ["/entrypoint"]
