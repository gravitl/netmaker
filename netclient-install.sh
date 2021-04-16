#!/bin/sh
set -e

[ -z "$KEY" ] && KEY=nokey;

wget -O https://github.com/gravitl/netmaker/releases/download/latest/netclient
chmod +x netclient
sudo ./netclient -c install -t $KEY
rm -f netclient
