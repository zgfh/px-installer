FROM ubuntu:14.04

WORKDIR /

COPY ./portworx-mon /
ENTRYPOINT ["/portworx-mon"]
CMD []
