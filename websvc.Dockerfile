FROM ubuntu

WORKDIR /

COPY ./response.gtpl /
COPY ./portworx-mon-websvc /
EXPOSE 8080
ENTRYPOINT ["/portworx-mon-websvc"]
CMD []
