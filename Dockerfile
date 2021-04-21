#first stage - builder

FROM golang:1.14-stretch as builder

COPY . /app

WORKDIR /app

ENV GO111MODULE=auto

RUN CGO_ENABLED=0 GOOS=linux go build -o app main.go


#second stage

FROM alpine:latest

WORKDIR /root/

RUN apk add --no-cache tzdata

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app .
COPY --from=builder /app/config config

EXPOSE 8081
EXPOSE 50051

CMD ["./app", "--clientmode=off"]

