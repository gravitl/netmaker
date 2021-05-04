#!/bin/sh
set -x

echo "Starting."

sudo docker kill mongodb || true
sudo docker rm mongodb || true
sudo docker volume rm mongovol || true
sudo docker kill coredns || true
sudo docker rm coredns || true
sudo docker kill netmaker-ui || true
sudo docker rm netmaker-ui || true
sudo netclient -c remove -n default || true
sudo rm -rf /etc/systemd/system/netmaker.service || true
sudo rm -rf /etc/netmaker || true
sudo rm -rf /usr/local/bin/netclient || true
sudo rm -rf /etc/netclient|| true
find  /etc/systemd/system/ -name 'netclient*' -exec rm {} \;
sudo systemctl daemon-reload || true
sudo systemctl enable systemd-resolved || true
sudo systemctl start systemd-resolved || true
sleep 5
sudo systemctl restart systemd-resolved || true

echo "Done."
