#!/bin/sh
set -x

echo "Starting."

sudo netclient -c remove-all || true
sudo rm -rf /usr/local/bin/netclient || true
sudo rm -rf /etc/netclient|| true
find  /etc/systemd/system/ -name 'netclient*' -exec rm {} \;
sudo systemctl daemon-reload || true

echo "Done."
