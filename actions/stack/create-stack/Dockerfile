FROM ubuntu:jammy
# We could not get this action working with alpine image
# we would get "file not found" errors trying to execute binaries like
# jam and pack, despite the fact they were installed correctly

RUN apt update && apt -y install curl jq

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
