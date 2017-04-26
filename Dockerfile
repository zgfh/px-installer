FROM ubuntu:14.04

WORKDIR /

COPY ./px-installer /
ENTRYPOINT ["/px-installer"]
CMD []
