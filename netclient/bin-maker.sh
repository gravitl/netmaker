#!/bin/bash
VERSION=${VERSION:-"develop"}
echo "build with version tag: $VERSION"
readonly __HOST_ARCH=${1:-"amd64"}  # change this for your machine.
readonly __HOST_GOOSE=${2:-"linux"} # change this for your machine.
readonly __EXEC_DIR=$(dirname "$(realpath $0)") && cd $__EXEC_DIR   

__darwin=( arm64 amd64 )
__linux=( amd64 arm arm64 mips mips64 mips64le mipsle ppc64 ppc64le riscv64 s390x 386 )
__freebsd=( amd64 arm arm64 386 )
__windows=( amd64 arm arm64 386 ) 

function build
{   
    local _goarch=${1:-"None"} && if [[ $_goarch == "None" ]]; then exit 1; fi
    local _goose="${2:-"None"}" && if [[ $_goose == "None" ]]; then exit 1; fi
    local _goarm=${3:-""}
    local _out=build/netclient-$_goose-$_goarch$_goarm && mkdir -p build
    if [ "$_goarch" == "arm" ] && [ "$_goarm" == "" ]; then
	    build $_goarch $_goose 5 && build $_goarch $_goose 6 && build $_goarch $_goose 7
    else
        
        if [[ $_goarch == mips* ]]; then
            #At present GOMIPS64 based binaries are not generated through this script, more details about GOMIPS environment variables in https://go.dev/doc/asm#mips .
            echo $_out-softfloat
            GOARM=$_goarm GOMIPS=softfloat GOARCH=$_goarch GOOS=$_goose GOHOSTARCH=$__HOST_ARCH CGO_ENABLED=0 go build -ldflags="-X 'main.version=$VERSION'" -o $_out-softfloat
            echo $_out
            GOARM=$_goarm GOARCH=$_goarch GOOS=$_goose GOHOSTARCH=$__HOST_ARCH CGO_ENABLED=0 go build -ldflags="-X 'main.version=$VERSION'" -o $_out
        else
            echo $_out
            GOARM=$_goarm GOARCH=$_goarch GOOS=$_goose GOHOSTARCH=$__HOST_ARCH CGO_ENABLED=0 go build -ldflags="-X 'main.version=$VERSION'" -o $_out
        fi
    fi
}

for arch in ${__linux[*]}; do build "$arch" "linux"; done

for arch in ${__freebsd[*]}; do build "$arch" "freebsd"; done

for arch in ${__darwin[*]}; do build "$arch" "darwin"; done

for arch in ${__windows[*]}; do build "$arch" "windows"; done
