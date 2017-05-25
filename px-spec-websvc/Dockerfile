FROM ubuntu

WORKDIR /

COPY ./templates/* /
#COPY ./templates/k8s-pxd-spec-response.gtpl /
COPY ./px-spec-websvc /
EXPOSE 8080
ENTRYPOINT ["/px-spec-websvc"]
CMD []
