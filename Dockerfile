#first stage - builder

FROM golang:latest as builder

COPY . /app

WORKDIR /app

ENV GO111MODULE=auto

RUN GOARCH=amd64 CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o app main.go

WORKDIR /app/netclient

ENV GO111MODULE=auto

RUN GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o netclient main.go

#second stage

FROM debian:latest

RUN apt-get update && apt-get -y install systemd procps

WORKDIR /root/

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app .
COPY --from=builder /app/config config
COPY --from=builder /app/netclient netclient

EXPOSE 8081
EXPOSE 50051

CMD ["./app"]
