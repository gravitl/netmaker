#!/bin/bash
echo "checking for root permissions..."

if [ $EUID -ne 0 ]; then
   echo "This script must be run as root" 
   exit 1
fi


echo "checking dependencies..."

declare -A osInfo;
osInfo[/etc/debian_version]="apt-get install -y"
osInfo[/etc/alpine-release]="apk --update add"
osInfo[/etc/centos-release]="yum install -y"
osInfo[/etc/fedora-release]="dnf install -y"

for f in ${!osInfo[@]}
do
    if [[ -f $f ]];then
        install_cmd=${osInfo[$f]}
    fi
done

dependencies=("docker" "docker-compose" "wireguard")

for dependency in ${dependencies[@]}; do
    is_installed=$(dpkg-query -W --showformat='${Status}\n' ${dependency} | grep "install ok installed")

    if [ "${is_installed}" == "install ok installed" ]; then
        echo "    " ${dependency} is installed
    else
            echo "    " ${dependency} is not installed. Attempting install.
            ${install_cmd} ${dependency}
            sleep 5
            is_installed=$(dpkg-query -W --showformat='${Status}\n' ${dependency} | grep "install ok installed")
            if [ "${is_installed}" == "install ok installed" ]; then
                echo "    " ${dependency} is installed
            elif [ -x "$(command -v ${dependency})" ]; then
                echo "    " ${dependency} is installed
            else
                echo "    " failed to install ${dependency}. Exiting.
                exit 1
            fi
    fi
done




set -e

echo "setting public ip values..."

NETMAKER_BASE_DOMAIN=nm.$(curl -s ifconfig.me | tr . -).nip.io
COREDNS_IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p')
SERVER_PUBLIC_IP=$(curl -s ifconfig.me)
REPLACE_MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')

echo "        domain: $NETMAKER_BASE_DOMAIN"
echo "    coredns ip: $COREDNS_IP"
echo "     public ip: $SERVER_PUBLIC_IP"
echo "    master key: $REPLACE_MASTER_KEY"


echo "creating caddyfile..."

cat >/root/Caddyfile<<EOL
{
    # LetsEncrypt account
    email fake@email.com
}

# Dashboard
https://dashboard.$NETMAKER_BASE_DOMAIN {
    reverse_proxy http://127.0.0.1:8082
}

# API
https://api.$NETMAKER_BASE_DOMAIN {
    reverse_proxy http://127.0.0.1:8081
}

# gRPC
https://grpc.$NETMAKER_BASE_DOMAIN {
    reverse_proxy h2c://127.0.0.1:50051
}
EOL


echo "creating docker-compose.yml..."

cat >/root/docker-compose.yml<<EOL
version: "3.4"

services:
  netmaker:
    container_name: netmaker
    image: gravitl/netmaker:v0.8.2
    volumes:
      - /etc/netclient/config:/etc/netclient/config
      - dnsconfig:/root/config/dnsconfig
      - /usr/bin/wg:/usr/bin/wg
      - sqldata:/root/data
    cap_add: 
      - NET_ADMIN
    restart: always
    network_mode: host
    environment:
      SERVER_HOST: "$SERVER_PUBLIC_IP"
      SERVER_API_CONN_STRING: "api.$NETMAKER_BASE_DOMAIN:443"
      SERVER_GRPC_CONN_STRING: "grpc.$NETMAKER_BASE_DOMAIN:443"
      COREDNS_ADDR: "$SERVER_PUBLIC_IP"
      GRPC_SSL: "on"
      DNS_MODE: "on"
      SERVER_HTTP_HOST: "api.$NETMAKER_BASE_DOMAIN"
      SERVER_GRPC_HOST: "grpc.$NETMAKER_BASE_DOMAIN"
      API_PORT: "8081"
      GRPC_PORT: "50051"
      CLIENT_MODE: "contained"
      MASTER_KEY: "REPLACE_MASTER_KEY"
      SERVER_GRPC_WIREGUARD: "off"
      CORS_ALLOWED_ORIGIN: "*"
      DATABASE: "sqlite"
  netmaker-ui:
    container_name: netmaker-ui
    depends_on:
      - netmaker
    image: gravitl/netmaker-ui:v0.8
    links:
      - "netmaker:api"
    ports:
      - "8082:80"
    environment:
      BACKEND_URL: "https://api.$NETMAKER_BASE_DOMAIN"
    restart: always
  coredns:
    depends_on:
      - netmaker 
    image: coredns/coredns
    command: -conf /root/dnsconfig/Corefile
    container_name: coredns
    restart: always
    ports:
      - "$COREDNS_IP:53:53/udp"
      - "$COREDNS_IP:53:53/tcp"
    volumes:
      - dnsconfig:/root/dnsconfig
  caddy:
    image: caddy:latest
    container_name: caddy
    restart: unless-stopped
    network_mode: host # Wants ports 80 and 443!
    volumes:
      - /root/Caddyfile:/etc/caddy/Caddyfile
      # - $PWD/site:/srv # you could also serve a static site in site folder
      - caddy_data:/data
      - caddy_conf:/config
volumes:
  caddy_data: {}
  caddy_conf: {}
  sqldata: {}
  dnsconfig: {}
EOL

echo "starting containers..."

docker-compose -f /root/docker-compose.yml up -d

sleep 5

echo "finished installing"

echo "visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
