FROM centos:8

# See https://hub.docker.com/repository/docker/hzgl/laitos for ready made images uploaded by the author of laitos program.
MAINTAINER Houzuo Guo <guohouzuo@gmail.com>

COPY laitos /laitos
ENTRYPOINT ["/laitos"]
