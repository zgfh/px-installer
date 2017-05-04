FROM ubuntu

WORKDIR /

COPY ./k8s-px-spec-response.gtpl /
COPY ./px-mon-websvc /
EXPOSE 8080
ENTRYPOINT ["/px-mon-websvc"]
CMD []
