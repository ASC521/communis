FROM docker.io/library/alpine:3.23.4

RUN mkdir -p /opt/communis/bin
RUN mkdir -p /etc/opt/communis
RUN mkdir -p /var/opt/communis

COPY dist/exec/communis /opt/communis/bin/
EXPOSE 6789
ENTRYPOINT ["/opt/communis/bin/communis"]
