#first stage - builder
FROM gravitl/builder as builder

WORKDIR /app

COPY . .

ENV GO111MODULE=auto

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=1 /usr/local/go/bin/go build -ldflags="-w -s" -o netmaker main.go

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
