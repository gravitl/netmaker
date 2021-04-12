#!/bin/sh
set -e

[ -z "$KEY" ] && KEY=nokey;

wget -O netclient https://github.com/gravitl/netmaker/releases/download/latest/netclient netclient
chmod +x netclient
sudo ./netclient -c install -s $SERVER_URL -g $NET_NAME -k $KEY
rm -f netclient
