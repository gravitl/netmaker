#first stage - builder
FROM golang:1.15-alpine as builder
ARG version
RUN apk add build-base
WORKDIR /app
COPY . .
ENV GO111MODULE=auto
RUN GOOS=linux CGO_ENABLED=1 go build -ldflags="-s -X 'main.version=$version'" -o netmaker main.go

FROM alpine:3.13.6
# add a c lib
RUN apk add gcompat iptables
# set the working directory
WORKDIR /root/
RUN mkdir -p /etc/netclient/config
COPY --from=builder /app/netmaker .
COPY --from=builder /app/config config
EXPOSE 8081
EXPOSE 50051
ENTRYPOINT ["./netmaker"]
