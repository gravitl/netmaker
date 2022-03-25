
SHELL := /bin/bash

default :: cat

install-go ::
	cat <(curl -sS https://git.sr.ht/~johns/install-go/blob/main/install-go) > install-go && chmod 755 install-go

fmt ::
	gofmt -w -s *.go
	gofmt -w -s */*.go
	gofmt -w -s */*/*.go
	gofmt -w -s */*/*/*.go

tidy :: fmt
	go mod tidy

install :: tidy
	go install ./...

proto :: 
	protoc --proto_path=grpc --go_out=grpc --go_opt=paths=source_relative grpc/node.proto

test :: install
	go test -v ./...

cat ::
	cat Makefile

all :: 
	./bin-maker.sh
	make -C netclient all

server :: 
	CGO_ENABLED=0 go build 

client :: 
	make -C netclient

format-patch ::
	git format-patch -1

apply-patch ::
	git apply --reject --whitespace=fix *.patch

.PHONY: default install-go fmt tidy cat all proto test client server format-patch apply-patch

# The above make targets can be used to compile various
# portions of netmaker.
#
# For example, to compile the server you will run:
# 	
# 	make server
#
