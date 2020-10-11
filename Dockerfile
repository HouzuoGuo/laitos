FROM ubuntu:20.04

# See https://hub.docker.com/repository/docker/hzgl/laitos for ready made images uploaded by the author of laitos program.
WORKDIR /
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt upgrade -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite && apt install -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite bind9-dnsutils busybox ca-certificates curl iputils-ping lftp net-tools netcat-openbsd socat wget

COPY laitos /laitos
ENTRYPOINT ["/laitos"]

# A gentle start - run laitos in a container and start the HTTP server:
# docker run -it --rm -p 12345:80 --env 'LAITOS_CONFIG={"HTTPFilters": {"PINAndShortcuts": {"Passwords": ["abcdefgh"]},"LintText": {"MaxLength": 1000}},"HTTPHandlers": {"CommandFormEndpoint": "/cmd"}}' hzgl/laitos:latest -daemons insecurehttpd
# Then you may visit the web server at "http://ContainerHost:12345/cmd", and enter an app command such as "abcdefg.s ls -l /".
