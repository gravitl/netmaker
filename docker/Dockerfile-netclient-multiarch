FROM golang:latest as builder
# add glib support daemon manager
WORKDIR /app
ARG version

COPY . .

ENV GO111MODULE=auto

RUN GOOS=linux CGO_ENABLED=0 /usr/local/go/bin/go build -ldflags="-w -s -X 'main.version=${TAG}'" -o netclient-app netclient/main.go

FROM alpine:3.13.6

WORKDIR /root/

RUN apk add --no-cache --update bash libmnl gcompat iptables openresolv iproute2 wireguard-tools 
COPY --from=builder /app/netclient-app ./netclient
COPY --from=builder /app/scripts/netclient.sh .
RUN chmod 0755 netclient && chmod 0755 netclient.sh


ENTRYPOINT ["/bin/sh", "./netclient.sh"]
