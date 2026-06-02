#!/bin/bash

#server build
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.version=$VERSION'" -o netclient/build/netmaker main.go

cd netclient
./bin-maker.sh

