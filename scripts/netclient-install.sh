#!/bin/sh
set -e

[ -z "$KEY" ] && KEY=nokey;

wget -O netclient https://github.com/gravitl/netmaker/releases/download/v0.5/netclient
chmod +x netclient
sudo ./netclient register -t $KEY
sudo ./netclient join -t $KEY
rm -f netclient
