FROM ubuntu:16.04

RUN apt-get clean && apt-get update && \
    apt-get install -y libltdl7 && \
    apt-get clean

ADD docker /usr/bin/docker
ENTRYPOINT [ "/usr/bin/docker" ]

