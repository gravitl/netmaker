
SHELL := /bin/bash

default :: cat

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

test :: install proto
	go test -v ./...

cat ::
	cat Makefile

all :: test
	./bin-maker.sh
	make -C netclient all

server :: test
	CGO_ENABLED=0 go build 

client :: test
	make -C netclient

format-patch ::
	git format-patch -1

apply-patch ::
	git apply --reject --whitespace=fix *.patch

# The above make targets can be used to compile various
# portions of netmaker.
#
# For example, to compile the server you will run:
# 	
# 	make server
#
