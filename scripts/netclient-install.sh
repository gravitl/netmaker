#!/bin/bash
set -e

if [ "$EUID" -ne 0 ]; then
   echo "This script must be run as root" 
   exit 1
fi

[ -z "$KEY" ] && KEY=nokey;
[ -z "$VERSION" ] && echo "no \$VERSION provided, fallback to latest" && VERSION=latest;

dist=netclient

echo "OS Version = $OSTYPE"
echo "Netclient Version = $VERSION"

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
	arch=$(uname -i)
	echo "CPU ARCH = $arch"
	if [ "$arch" == 'x86_64' ];
	then 
		dist=netclient 
	fi
	if [ "$arch" == 'x86_32' ];
	then
		dist=netclient-32
	fi
	if [ "$arch" == 'armv*' ];
	then
		dist=netclient-arm64
	fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
        dist=netclient-darwin
else
        echo "This OS is not currently supported via automated install" 
        exit 1
fi

echo "Binary = $dist"

wget -O netclient https://github.com/gravitl/netmaker/releases/download/$VERSION/netclient
chmod +x netclient
sudo ./netclient join -t $KEY
rm -f netclient
