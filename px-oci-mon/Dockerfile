FROM alpine:latest

RUN apk add --update util-linux
ADD px-oci-mon /
ENTRYPOINT [ "/px-oci-mon" ]

