#!/bin/sh
set -e

[ -z "$SERVER_URL" ] && echo "Need to set SERVER_URL" && exit 1;
[ -z "$NETWORK_NAME" ] && echo "Need to set NETWORK_NAME" && exit 1;
[ -z "$KEY_VALUE" ] && KEY_VALUE=nokey;



wget -O netclient https://github.com/gravitl/netmaker/releases/download/v0.1/netclient
chmod +x netclient
sudo ./netclient -c install -s $SERVER_URL -g $NETWORK_NAME -k $KEY_VALUE
rm -f netclient
