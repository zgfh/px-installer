FROM fedora:25

WORKDIR /

COPY ./portworx-mon /
ENTRYPOINT ["/portworx-mon"]
CMD []
