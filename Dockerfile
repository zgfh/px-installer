FROM fedora:25

WORKDIR /

COPY ./px-mon /
ENTRYPOINT ["/px-mon"]
CMD []
