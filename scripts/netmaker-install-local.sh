#!/bin/sh
set -x

[ -z "$SERVER_DOMAIN" ] && echo "Need to set SERVER_DOMAIN (format: 1.2.3.4 or mybackend.com)" && exit 1;


install() {

docker volume create mongovol && docker run -d --name mongodb -v mongovol:/data/db -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=mongoadmin -e MONGO_INITDB_ROOT_PASSWORD=mongopass mongo 

echo "Giving Mongo Time to Start"
sleep 10
echo "Installing Netmaker API"

mkdir -p /etc/netmaker/config/environments
mkdir -p /etc/netmaker/config/dnsconfig
cp ../netmaker /etc/netmaker/netmaker
chmod +x /etc/netmaker/netmaker


cat >/etc/netmaker/config/environments/dev.yaml<<EOL
server:
  host: "$SERVER_DOMAIN"
  apiport: "8081"
  grpcport: "50051"
  masterkey: "secretkey"
  allowedorigin: "*"
  restbackend: true            
  agentbackend: true
  defaultnetname: "default"
  defaultnetrange: "10.10.10.0/24"
  createdefault: true
mongoconn:
  user: "mongoadmin"
  pass: "mongopass"
  host: "127.0.0.1"
  port: "27017"
  opts: '/?authSource=admin'
EOL

cat >/etc/netmaker/config/dnsconfig/Corefile<<EOL
. {
    hosts ./root/netmaker.hosts {
	fallthrough	
    }
    forward . 8.8.8.8 8.8.4.4
    log
}
EOL

cat >/etc/systemd/system/netmaker.service<<EOL
[Unit]
Description=Netmaker Server
After=network.target

[Service]
Type=simple
Restart=on-failure

WorkingDirectory=/etc/netmaker
ExecStart=/etc/netmaker/netmaker

[Install]
WantedBy=multi-user.target
EOL
systemctl daemon-reload
systemctl start netmaker.service
sudo docker pull coredns/coredns
sudo docker pull gravitl/netmaker-ui:v0.3

systemctl stop systemd-resolved
systemctl disable systemd-resolved
echo "Running CoreDNS"
sudo docker run -d --name coredns --restart=always --volume=/etc/netmaker/config/dnsconfig/:/root/ -p 53:53/udp coredns/coredns -conf /root/Corefile

echo "Running UI"
sudo docker run -d --name netmaker-ui -p 80:80 -e BACKEND_URL="http://$SERVER_DOMAIN:8081" gravitl/netmaker-ui:v0.3

echo "Setup Complete"
}

cleanup() {
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
sudo systemctl enable systemd-resolved
sudo systemctl start systemd-resolved
sleep 5
sudo systemctl restart systemd-resolved
}

trap cleanup ERR
cleanup
install
