FROM debian:stable-slim as base

RUN apt-get update && apt-get -uy upgrade
RUN apt-get -y install ca-certificates && update-ca-certificates
# install golang

ENV GO_VERSION=1.16
ENV OS=linux
ENV ARCH=amd64

FROM base as build

RUN apt-get install -y git wget 
RUN \
    wget https://golang.org/dl/go${GO_VERSION}.${OS}-${ARCH}.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.${OS}-${ARCH}.tar.gz && \
    rm go${GO_VERSION}.${OS}-${ARCH}.tar.gz

ENV COREDNS_VERSION=v1.8.6

RUN \
    mkdir -p /go/src/github.com/coredns && \
    cd /go/src/github.com/coredns && \
    wget https://github.com/coredns/coredns/archive/refs/tags/${COREDNS_VERSION}.tar.gz && \
    tar xvf ${COREDNS_VERSION}.tar.gz && \
    mv coredns* coredns

WORKDIR /go/src/github.com/coredns/coredns
COPY coredns/plugin plugin/netmaker
COPY . netmaker

RUN \
    export PATH=$PATH:/usr/local/go/bin/ && \
    sed '/file:file/i netmaker:netmaker' plugin.cfg -i && \
    go generate && \
    go mod tidy && \
    go build -o /coredns

FROM base

COPY --from=build /coredns /coredns

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]