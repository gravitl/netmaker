#first stage - builder
FROM gravitl/go-builder AS builder
ARG tags 
WORKDIR /app
COPY . .

RUN GOOS=linux CGO_ENABLED=1 go build -ldflags="-s -w " -tags ${tags} .
# RUN go build -tags=ee . -o netmaker main.go
FROM alpine:3.20.0

# add a c lib
# set the working directory
WORKDIR /root/
RUN mkdir -p /etc/netclient/config
COPY --from=builder /app/netmaker .
COPY --from=builder /app/config config
EXPOSE 8081
ENTRYPOINT ["./netmaker"]
