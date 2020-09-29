FROM ubuntu:20.04

# See https://hub.docker.com/repository/docker/hzgl/laitos for ready made images uploaded by the author of laitos program.
WORKDIR /
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt upgrade -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite && apt install -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite ca-certificates

COPY laitos /laitos
ENTRYPOINT ["/laitos"]
