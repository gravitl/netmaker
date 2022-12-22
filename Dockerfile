#first stage - builder
FROM gravitl/go-builder as builder
ARG version
ARG tags 
WORKDIR /app
COPY . .
ENV GO111MODULE=auto

RUN apk add git
RUN GOOS=linux CGO_ENABLED=1 go build -ldflags="-s -X 'main.version=${version}'" -tags ${tags} .
# RUN go build -tags=ee . -o netmaker main.go
FROM alpine:3.16.2

# add a c lib
RUN apk add gcompat iptables wireguard-tools
# set the working directory
WORKDIR /root/
RUN mkdir -p /etc/netclient/config
COPY --from=builder /app/netmaker .
COPY --from=builder /app/config config
EXPOSE 8081
ENTRYPOINT ["./netmaker"]
