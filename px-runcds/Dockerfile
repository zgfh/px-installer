FROM alpine:latest

RUN apk add --update util-linux
ADD px-runcds /
ENTRYPOINT [ "/px-runcds" ]

