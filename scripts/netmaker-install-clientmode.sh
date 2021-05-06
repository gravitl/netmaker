#!/bin/sh
set -e

[ -z "$SERVER_DOMAIN" ] && echo "Need to set SERVER_DOMAIN (format: 1.2.3.4 or mybackend.com)" && exit 1;


docker volume create mongovol && docker run -d --name mongodb -v mongovol:/data/db --network host -e MONGO_INITDB_ROOT_USERNAME=mongoadmin -e MONGO_INITDB_ROOT_PASSWORD=mongopass mongo --bind_ip 0.0.0.0 

mkdir -p /etc/netmaker/config/environments
wget -O /etc/netmaker/netmaker https://github.com/gravitl/netmaker/releases/download/latest/netmaker
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
  host: "localhost"
  port: "27017"
  opts: '/?authSource=admin'
EOL

cat >/etc/netmaker/config/Corefile<<EOL
. {
    hosts /root/netmaker.hosts
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


docker run -d --name netmaker-ui -p 80:80 -e BACKEND_URL="http://$SERVER_DOMAIN:8081" gravitl/netmaker-ui:v0.2
docker run -d --name coredns --restart=always --volume=/etc/netmaker/config/:/root/ -p 52:53/udp coredns/coredns -conf /root/Corefile
