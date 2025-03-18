#first stage - builder
FROM gravitl/go-builder:1.23.0 AS builder
ARG tags 
WORKDIR /app
COPY . .

RUN GOOS=linux CGO_ENABLED=1 go build -race -ldflags="-s -w " -tags ${tags} .
# RUN go build -tags=ee . -o netmaker main.go
FROM alpine:3.21.2

# add a c lib
# set the working directory
WORKDIR /root/
RUN apk update && apk upgrade
RUN apk add --no-cache sqlite
RUN mkdir -p /etc/netclient/config
COPY --from=builder /app/netmaker .
COPY --from=builder /app/config config
EXPOSE 8081
ENTRYPOINT ["./netmaker"]
