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

dependencies=("docker.io" "docker-compose" "wireguard")

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
MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
EMAIL="fake@email.com"

arg1=$( echo $1 | awk -F"domain=" '{print $2}')
arg2=$( echo $2 | awk -F"domain=" '{print $2}')

if [ -n "$arg1" ]; then
  echo "Parameter NETMAKER_BASE_DOMAIN is $arg1"
  NETMAKER_BASE_DOMAIN=$arg1

elif [ -n "$arg2" ]; then
  echo "Parameter NETMAKER_BASE_DOMAIN is $arg2"
  NETMAKER_BASE_DOMAIN=$arg2
fi

arg1=$( echo $1 | awk -F"email=" '{print $2}')
arg2=$( echo $2 | awk -F"email=" '{print $2}')

if [ -n "$arg1" ]; then
  echo "Parameter EMAIL is $arg1"
  EMAIL=$arg1

elif [ -n "$arg2" ]; then
  echo "Parameter EMAIL is $arg2"
  EMAIL=$arg2
fi

echo "        domain: $NETMAKER_BASE_DOMAIN"
echo "    coredns ip: $COREDNS_IP"
echo "     public ip: $SERVER_PUBLIC_IP"
echo "    master key: $MASTER_KEY"


echo "setting caddyfile..."


wget -q -O /root/Caddyfile https://raw.githubusercontent.com/gravitl/netmaker/master/docker/Caddyfile
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/Caddyfile
sed -i "s/YOUR_EMAIL/$EMAIL/g" /root/Caddyfile


echo "setting docker-compose..."

wget -q -O /root/docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.caddy.yml
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/docker-compose.yml
sed -i "s/SERVER_PUBLIC_IP/$SERVER_PUBLIC_IP/g" /root/docker-compose.yml
sed -i "s/COREDNS_IP/$COREDNS_IP/g" /root/docker-compose.yml
sed -i "s/REPLACE_MASTER_KEY/$MASTER_KEY/g" /root/docker-compose.yml

echo "starting containers..."

docker-compose -f /root/docker-compose.yml up -d

cat << "EOF"


    ______     ______     ______     __   __   __     ______   __                        
   /\  ___\   /\  == \   /\  __ \   /\ \ / /  /\ \   /\__  _\ /\ \                       
   \ \ \__ \  \ \  __<   \ \  __ \  \ \ \'/   \ \ \  \/_/\ \/ \ \ \____                  
    \ \_____\  \ \_\ \_\  \ \_\ \_\  \ \__|    \ \_\    \ \_\  \ \_____\                 
     \/_____/   \/_/ /_/   \/_/\/_/   \/_/      \/_/     \/_/   \/_____/                 
                                                                                         
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                         															 

EOF


echo "visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
echo""
sleep 2

if [ "${NETWORK_SETUP}" == "off" ]; then
	echo "install complete"
	exit 0
fi

echo "creating default network (10.101.0.0/16)"

curl -d '{"addressrange":"10.101.0.0/16","netid":"default"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' localhost:8081/api/networks

sleep 2

echo "creating default key"

curlresponse=$(curl -s -d '{"uses":99999,"name":"defaultkey"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' localhost:8081/api/networks/default/keys)
ACCESS_TOKEN=$(jq -r '.accessstring' <<< ${curlresponse})

sleep 2

echo "configuring netmaker server as ingress gateway"

curlresponse=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' localhost:8081/api/nodes/default)
SERVER_ID=$(jq -r '.[0].macaddress' <<< ${curlresponse})

curl -X POST -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' localhost:8081/api/nodes/default/$SERVER_ID/createingress

echo "finished configuring server and network. You can now add clients."
echo ""
echo ""
echo "For Linux and Mac clients, install with the following command:"
echo "        curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/develop/scripts/netclient-install.sh | sudo KEY=$ACCESS_TOKEN sh -"
echo ""
echo ""
echo "For Windows clients, perform the following from powershell, as administrator:"
echo "        1. Make sure WireGuardNT is installed - https://download.wireguard.com/windows-client/wireguard-installer.exe"
echo "        2. Download netclient.exe - wget https://github.com/gravitl/netmaker/releases/download/latest/netclient.exe"
echo "        3. Install Netclient - powershell.exe .\\netclient.exe join -t $ACCESS_TOKEN"
echo "        4. Whitelist C:\ProgramData\Netclient in Windows Defender"
echo ""
echo ""
echo "For Android and iOS clients, perform the following steps:"
echo "        1. Log into UI at dashboard.$NETMAKER_BASE_DOMAIN"
echo "        2. Navigate to \"EXTERNAL CLIENTS\" tab"
echo "        3. Select the gateway and create clients"
echo "        4. Scan the QR Code from WireGuard app in iOS or Android"
echo ""
echo ""
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
