FROM ubuntu

WORKDIR /

COPY ./k8s-pxd-spec-response.gtpl /
COPY ./px-spec-websvc /
EXPOSE 8080
ENTRYPOINT ["/px-spec-websvc"]
CMD []
