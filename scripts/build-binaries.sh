#!/bin/bash

#server build
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.version=$VERSION'" -o netclient/build/netmaker main.go

cd netclient

#client build
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-arm5 main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-arm6 main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-arm7 main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-arm64 main.go
env CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-s -w -X 'main.version=$VERSION'" -o build/netclient-mipsle main.go && upx build/netclient-mipsle
env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build  -ldflags="-X 'main.version=$VERSION'" -o build/netclient-freebsd main.go
env CGO_ENABLED=0 GOOS=freebsd GOARCH=arm GOARM=5 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-freebsd-arm5 main.go
env CGO_ENABLED=0 GOOS=freebsd GOARCH=arm GOARM=6 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-freebsd-arm6 main.go
env CGO_ENABLED=0 GOOS=freebsd GOARCH=arm GOARM=7 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-freebsd-arm7 main.go
env CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-freebsd-arm64 main.go
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.version=$VERSION'" -o build/netclient-darwin main.go
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'main.version=${VERSION}'" -o build/netclient-darwin-arm64 main.go