
SHELL := /bin/bash

default :: cat

fmt ::
	gofmt -w -s *.go
	gofmt -w -s */*.go
	gofmt -w -s */*/*.go
	gofmt -w -s */*/*/*.go

test ::
	go test -v ./...

cat ::
	cat Makefile

all ::  
	./bin-maker.sh

server ::
	CGO_ENABLED=0 go build 

client ::
	make -C netclient

client-all ::
	make -C netclient all

format-patch ::
	git format-patch -1

apply-patch ::
	git apply --reject --whitespace=fix *.patch

proto ::
	protoc --proto_path=grpc --go_out=grpc --go_opt=paths=source_relative grpc/node.proto

# The above make targets can be used to compile various
# portions of netmaker.
#
# For example, to compile the server you will run:
# 	
# 	make server
#
