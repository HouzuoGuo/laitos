FROM ubuntu:20.04

# See https://hub.docker.com/repository/docker/hzgl/laitos for ready made images uploaded by the author of laitos program.
WORKDIR /
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt upgrade -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite && apt install -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite bind9-dnsutils busybox ca-certificates curl iputils-ping lftp net-tools netcat-openbsd socat wget

COPY laitos /laitos
ENTRYPOINT ["/laitos"]

# A gentle start - run laitos in a container and start the HTTP server:
# 1. Start the container: docker run -it --rm -p 12345:80 --env 'LAITOS_CONFIG={"HTTPFilters": {"PINAndShortcuts": {"Passwords": ["abcdefgh"]},"LintText": {"MaxLength": 1000}},"HTTPHandlers": {"AppCommandEndpoint": "/cmd"}}' hzgl/laitos:latest -daemons insecurehttpd
# 2. In browser window, navigate to "http://server-ip:80/cmd?cmd=abcdefgh.s date", the example command calls for shell app ".s" to print out the system date and time.
# 3. The browser page will display the app command result on the page.
