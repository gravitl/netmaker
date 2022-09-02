#!/bin/bash
echo "checking for root permissions..."

echo "setting flags..."

while getopts d:e:m:v:c: flag
do
    case "${flag}" in
    	d) domain=${OPTARG};;
        e) email=${OPTARG};;
        m) addmesh=${OPTARG};;
        v) addvpn=${OPTARG};;
        c) num_clients=${OPTARG};;
    esac
done

echo "checking for root permissions..."


if [ $EUID -ne 0 ]; then
   echo "This script must be run as root" 
   exit 1
fi




echo "checking dependencies..."

declare -A osInfo;
osInfo[/etc/debian_version]="apt-get install -y"u
osInfo[/etc/alpine-release]="apk --update add"
osInfo[/etc/centos-release]="yum install -y"
osInfo[/etc/fedora-release]="dnf install -y"

for f in ${!osInfo[@]}
do
    if [[ -f $f ]];then
        install_cmd=${osInfo[$f]}
    fi
done

if [ -f /etc/debian_version ]; then
	apt update
elif [ -f /etc/alpine-release ]; then
  apk update
elif [ -f /etc/centos-release ]; then
	yum update
elif [ -f /etc/fedora-release ]; then
	dnf update
fi

dependencies=( "docker.io" "docker-compose" "wireguard" "jq" )

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

NETMAKER_BASE_DOMAIN=nm.$(curl -s ifconfig.me | tr . -).nip.io
COREDNS_IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p')
SERVER_PUBLIC_IP=$(curl -s ifconfig.me)
MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
EMAIL="$(echo $RANDOM | md5sum  | head -c 32)@email.com"

if [ -n "$domain" ]; then
  NETMAKER_BASE_DOMAIN=$domain
fi

if [ -n "$email" ]; then
  EMAIL=$email
fi

if [ -n "$addmesh" ]; then
  MESH_SETUP=$addmesh
else
  MESH_SETUP="true"
fi

if [ -n "$addvpn" ]; then
  VPN_SETUP=$addvpn
else
  VPN_SETUP="false"
fi

if [ -n "$num_clients" ]; then
  NUM_CLIENTS=$num_clients
else
  NUM_CLIENTS=5
fi


echo "   ----------------------------"
echo "                SETUP ARGUMENTS"
echo "   ----------------------------"
echo "        domain: $NETMAKER_BASE_DOMAIN"
echo "         email: $EMAIL"
echo "    coredns ip: $COREDNS_IP"
echo "     public ip: $SERVER_PUBLIC_IP"
echo "    master key: $MASTER_KEY"
echo "   setup mesh?: $MESH_SETUP"
echo "    setup vpn?: $VPN_SETUP"
if [ "${VPN_SETUP}" == "true" ]; then
echo "     # clients: $NUM_CLIENTS"
fi
echo "   ----------------------------"

sleep 5

echo "setting mosquitto.conf..."

wget -q -O /root/mosquitto.conf https://raw.githubusercontent.com/gravitl/netmaker/master/docker/mosquitto.conf

echo "setting docker-compose..."

mkdir -p /etc/netmaker

wget -q -O /root/docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.yml
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/docker-compose.yml
sed -i "s/SERVER_PUBLIC_IP/$SERVER_PUBLIC_IP/g" /root/docker-compose.yml
sed -i "s/COREDNS_IP/$COREDNS_IP/g" /root/docker-compose.yml
sed -i "s/REPLACE_MASTER_KEY/$MASTER_KEY/g" /root/docker-compose.yml
sed -i "s/YOUR_EMAIL/$EMAIL/g" /root/docker-compose.yml

echo "starting containers..."

docker-compose -f /root/docker-compose.yml up -d

test_connection() {

echo "testing Traefik setup (please be patient, this may take 1-2 minutes)"
for i in 1 2 3 4 5 6
do
curlresponse=$(curl -vIs https://api.${NETMAKER_BASE_DOMAIN} 2>&1)

if [[ "$i" == 6 ]]; then
  echo "    Traefik is having an issue setting up certificates, please investigate (docker logs traefik)"
  echo "    exiting..."
  exit 1
elif [[ "$curlresponse" == *"failed to verify the legitimacy of the server"* ]]; then
  echo "    certificates not yet configured, retrying..."

elif [[ "$curlresponse" == *"left intact"* ]]; then
  echo "    certificates ok"
  break
else
  secs=$(($i*5+10))
  echo "    issue establishing connection...retrying in $secs seconds..."       
fi
sleep $secs
done
}

set +e
test_connection


cat << "EOF"

                                                                                         
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                         													 

EOF


echo "visit https://dashboard.$NETMAKER_BASE_DOMAIN to log in"
sleep 7

setup_mesh() {( set -e
echo "creating netmaker network (10.101.0.0/16)"

curl -s -o /dev/null -d '{"addressrange":"10.101.0.0/16","netid":"netmaker"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/networks

sleep 5

echo "creating netmaker access key"

curlresponse=$(curl -s -d '{"uses":99999,"name":"netmaker-key"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/networks/netmaker/keys)
ACCESS_TOKEN=$(jq -r '.accessstring' <<< ${curlresponse})

sleep 5

echo "configuring netmaker server as ingress gateway"

curlresponse=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/netmaker)
SERVER_ID=$(jq -r '.[0].id' <<< ${curlresponse})

curl -o /dev/null -s -X POST -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/netmaker/$SERVER_ID/createingress

sleep 5

echo "finished configuring server and network. You can now add clients."
echo ""
echo "For Linux, Mac, Windows, and FreeBSD:"
echo "        1. Install the netclient: https://docs.netmaker.org/netclient.html#installation"
echo "        2. Join the network: netclient join -t $ACCESS_TOKEN"
echo ""
echo "For Android and iOS clients, perform the following steps:"
echo "        1. Log into UI at dashboard.$NETMAKER_BASE_DOMAIN"
echo "        2. Navigate to \"EXTERNAL CLIENTS\" tab"
echo "        3. Select the gateway and create clients"
echo "        4. Scan the QR Code from WireGuard app in iOS or Android"
echo ""
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
)}

setup_vpn() {( set -e
echo "creating vpn network (10.201.0.0/16)"

curl -s -o /dev/null -d '{"addressrange":"10.201.0.0/16","netid":"vpn","defaultextclientdns":"8.8.8.8"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/networks

sleep 5

echo "configuring netmaker server as vpn inlet..."

curlresponse=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/vpn)
SERVER_ID=$(jq -r '.[0].id' <<< ${curlresponse})

curl -s -o /dev/null -X POST -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/vpn/$SERVER_ID/createingress

echo "waiting 10 seconds for server to apply configuration..."

sleep 10


echo "configuring netmaker server vpn gateway..."

[ -z "$GATEWAY_IFACE" ] && GATEWAY_IFACE=$(ip -4 route ls | grep default | grep -Po '(?<=dev )(\S+)')

echo "gateway iface: $GATEWAY_IFACE"

curlresponse=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/vpn)
SERVER_ID=$(jq -r '.[0].id' <<< ${curlresponse})

EGRESS_JSON=$( jq -n \
                  --arg gw "$GATEWAY_IFACE" \
                  '{ranges: ["0.0.0.0/0","::/0"], interface: $gw}' )

echo "egress json: $EGRESS_JSON"
curl -s -o /dev/null -X POST -d "$EGRESS_JSON" -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/vpn/$SERVER_ID/creategateway

echo "creating client configs..."

for ((a=1; a <= $NUM_CLIENTS; a++))
do
        CLIENT_JSON=$( jq -n \
                  --arg clientid "vpnclient-$a" \
                  '{clientid: $clientid}' )

        curl -s -o /dev/null -d "$CLIENT_JSON" -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/extclients/vpn/$SERVER_ID
done

echo "finished configuring vpn server."
echo ""
echo "To configure clients, perform the following steps:"
echo "        1. log into dashboard.$NETMAKER_BASE_DOMAIN"
echo "        2. Navigate to \"EXTERNAL CLIENTS\" tab"
echo "        3. Download or scan a client config (vpnclient-x) to the appropriate device"
echo "        4. Follow the steps for your system to configure WireGuard on the appropriate device"
echo "        5. Create and delete clients as necessary. Changes to netmaker server settings require regenerating ext clients."

)}

if [ "${MESH_SETUP}" != "false" ]; then
        setup_mesh
fi

if [ "${VPN_SETUP}" == "true" ]; then
        setup_vpn
fi

echo ""
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
