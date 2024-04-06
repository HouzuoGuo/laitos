FROM ubuntu:latest

# See https://hub.docker.com/repository/docker/hzgl/laitos for ready made images uploaded by the author of laitos program.
WORKDIR /
ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt upgrade -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite && apt install -q -y -f -m -o Dpkg::Options::=--force-confold -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-overwrite bind9-dnsutils busybox ca-certificates curl iputils-ping lftp links2 net-tools netcat-openbsd socat wget

COPY laitos.amd64 /laitos.amd64
ENTRYPOINT ["/laitos.amd64"]

# Give this a try - start the laitos web server (HTTP) with a couple of web services:
# docker run -it --rm -p 12345:80 --env 'LAITOS_CONFIG={"HTTPFilters": {"PINAndShortcuts": {"Passwords": ["password"]}, "LintText": {"MaxLength": 1000}}, "HTTPHandlers": {"CommandFormEndpoint": "/cmd", "FileUploadEndpoint": "/upload", "InformationEndpoint": "/info", "LatestRequestsInspectorEndpoint": "/latest_requests", "ProcessExplorerEndpoint": "/proc", "RequestInspectorEndpoint": "/myrequest", "WebProxyEndpoint": "/proxy"}}' --env 'LAITOS_INDEX_PAGE=Welcome to laitos, try these out: /cmd /upload /info /latest_requests?e=1 /proc?pid=0 /myrequest /proxy?u=http://google.com' hzgl/laitos:latest -daemons insecurehttpd
