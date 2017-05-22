FROM ubuntu

WORKDIR /

COPY ./templates/swarm-px-service-spec-response.gtpl /
COPY ./templates/k8s-pxd-spec-response.gtpl /
COPY ./px-spec-websvc.out /
EXPOSE 8080
ENTRYPOINT ["/px-spec-websvc.out"]
CMD []
