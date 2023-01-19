#!/bin/bash

cat << "EOF"
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
                                                                                         
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                                                                                                                                 

- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
EOF

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

wait_seconds() {(
  for ((a=1; a <= $1; a++))
  do
    echo ". . ."
    sleep 1
  done
)}

confirm() {(
  while true; do
      read -p 'Does everything look right? [y/n]: ' yn
      case $yn in
          [Yy]* ) override="true"; break;;
          [Nn]* ) echo "exiting..."; exit 1;;
          * ) echo "Please answer yes or no.";;
      esac
  done
)}

echo "checking dependencies..."

OS=$(uname)

if [ -f /etc/debian_version ]; then
	dependencies="yq jq"
	update_cmd='apt update'
	install_cmd='apt-get install -y'
elif [ -f /etc/centos-release ]; then
	dependencies="wireguard jq docker.io docker-compose netclient"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/fedora-release ]; then
	dependencies="wireguard jq docker.io docker-compose netclient"
	update_cmd='dnf update'
	install_cmd='dnf install -y'
elif [ -f /etc/redhat-release ]; then
	dependencies="wireguard jq docker.io docker-compose netclient"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/arch-release ]; then
    	dependecies="wireguard-tools jq docker.io docker-compose netclient"
	update_cmd='pacman -Sy'
	install_cmd='pacman -S --noconfirm'
else
	echo "OS not supported for automatic install"
    exit 1
fi

set -- $dependencies

${update_cmd}

while [ -n "$1" ]; do
    is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
    if [ "${is_installed}" != "" ]; then
        echo "    " $1 is installed
    else
        echo "    " $1 is not installed. Attempting install.
        ${install_cmd} $1
        sleep 5
        if [ "${OS}" = "OpenWRT" ] || [ "${OS}" = "TurrisOS" ]; then
            is_installed=$(opkg list-installed $1 | grep $1)
        else
            is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
        fi
        if [ "${is_installed}" != "" ]; then
            echo "    " $1 is installed
        elif [ -x "$(command -v $1)" ]; then
            echo "  " $1 is installed
        else
            echo "  " FAILED TO INSTALL $1
            echo "  " This may break functionality.
        fi
    fi
	shift
done

echo "-----------------------------------------------------"
echo "dependency check complete"
echo "-----------------------------------------------------"

wait_seconds 3

set -e

unset MASTER_KEY
MASTER_KEY=$(yq -r .services.netmaker.environment.MASTER_KEY docker-compose.yml)
echo "-----------------------------------------------------"
echo "Is $MASTER_KEY the correct master key for your Netmaker installation?"
echo "-----------------------------------------------------"
select mkey_option in "yes" "no (enter manually)"; do
  case $REPLY in
    1)
      echo "using $MASTER_KEY for master key"
	  break
      ;;      
    2)
      read -p "Enter Master Key: " mkey
      MASTER_KEY=$mkey
      echo "using $MASTER_KEY"
      break
      ;;
    *) echo "invalid option $REPLY";;
  esac
done

unset SERVER_HTTP_HOST
SERVER_HTTP_HOST=$(yq -r .services.netmaker.environment.SERVER_HTTP_HOST docker-compose.yml)
echo "-----------------------------------------------------"
echo "Is $SERVER_HTTP_HOST the correct endpoint for your Netmaker installation?"
echo "-----------------------------------------------------"
select endpoint_option in "yes" "no (enter manually)"; do
  case $REPLY in
    1)
      echo "using $SERVER_HTTP_HOST for endpoint"
	  break
      ;;      
    2)
      read -p "Enter Endpoint: " endpoint
      SERVER_HTTP_HOST=$endpoint
      echo "using $SERVER_HTTP_HOST"
      break
      ;;
    *) echo "invalid option $REPLY";;
  esac
done


unset BROKER_NAME
BROKER_NAME=$(yq -r .services.netmaker.environment.SERVER_NAME docker-compose.yml)
echo "-----------------------------------------------------"
echo "Is $BROKER_NAME the correct domain for your MQ broker?"
echo "-----------------------------------------------------"
select broker_option in "yes" "no (enter manually)"; do
  case $REPLY in
    1)
      echo "using $BROKER_NAME for endpoint"
	  break
      ;;      
    2)
      read -p "Enter Broker Domain: " broker
      BROKER_NAME=$broker
      echo "using $BROKER_NAME"
      break
      ;;
    *) echo "invalid option $REPLY";;
  esac
done


CURRENT_VERSION=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://$SERVER_HTTP_HOST/api/server/getserverinfo | jq ' .Version')

if [[ $CURRENT_VERSION == '"v0.17.1"' ]]; then
    echo "version is $CURRENT_VERSION"
else
    echo "error, current version is $CURRENT_VERSION"
    echo "please upgrade to v0.17.1 in order to use the upgrade script"
    exit 1
fi

curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://$SERVER_HTTP_HOST/api/nodes | jq -c '[ .[] | select(.isserver=="yes") ]' > nodejson
NODE_LEN=$(jq length nodejson.tmp)
HAS_INGRESS="no"
echo $NODE_LEN
if [ "$NODE_LEN" -gt 0 ]; then
    echo "===SERVER NODES==="
    for i in $(seq 1 $NODE_LEN); do
        NUM=$(($i-1))
        echo "  SERVER NODE $NUM:"
        echo "    network: $(jq ".[$NUM].network" ./nodejson.tmp)"
        echo "      name: $(jq ".[$NUM].name" ./nodejson.tmp)"
        echo "      private ipv4: $(jq ".[$NUM].address" ./nodejson.tmp)"
        echo "      private ipv6: $(jq ".[$NUM].address6" ./nodejson.tmp)"
        echo "      is egress: $(jq ".[$NUM].isegressgateway" ./nodejson.tmp)"
        if [[ $(jq ".[$NUM].isegressgateway" ./nodejson.tmp) == "yes" ]]; then
            echo "          egress range: $(jq ".[$NUM].egressgatewayranges" ./nodejson.tmp)"
        fi
        echo "      is ingress: $(jq ".[$NUM].isingressgateway" ./nodejson.tmp)"
        if [[ $(jq ".[$NUM].isingressgateway" ./nodejson.tmp) == "yes" ]]; then
            HAS_INGRESS="yes"
        fi
        echo "      is relay: $(jq ".[$NUM].isrelay" ./nodejson.tmp)"
        echo "      is failover: $(jq ".[$NUM].failover" ./nodejson.tmp)"
        echo "  ------------"
    done
    echo "=================="
else
    echo "no nodes to parse"
fi

echo "Please confirm that the above output matches the server nodes in your Netmaker server."
confirm


if [[ $HAS_INGRESS == "yes" ]]; then
    echo "WARNING: Your server contains an Ingress Gateway. After upgrading, existing Ext Clients will be lost and must be recreated. Please confirm that you would like to continue."
    confirm
fi

echo "Setting docker-compose and Caddyfile..."

sed -i "s/v0.17.1/v0.18.0/g" /root/docker-compose.yml


sed -i "s/v0.17.1/v0.18.0/g" /root/docker-compose.yml
SERVER_NAME: "nm.167-71-28-181.nip.io"

echo "Starting containers..."

docker-compose -f /root/docker-compose.yml up -d

sleep 2

test_connection() {

echo "Testing Caddy setup (please be patient, this may take 1-2 minutes)"
for i in 1 2 3 4 5 6 7 8
do
curlresponse=$(curl -vIs https://api.${NETMAKER_BASE_DOMAIN} 2>&1)

if [[ "$i" == 8 ]]; then
  echo "    Caddy is having an issue setting up certificates, please investigate (docker logs caddy)"
  echo "    Exiting..."
  exit 1
elif [[ "$curlresponse" == *"failed to verify the legitimacy of the server"* ]]; then
  echo "    Certificates not yet configured, retrying..."

elif [[ "$curlresponse" == *"left intact"* ]]; then
  echo "    Certificates ok"
  break
else
  secs=$(($i*5+10))
  echo "    Issue establishing connection...retrying in $secs seconds..."       
fi
sleep $secs
done
}


setup_netclient() {( set -e 
if [ -f /etc/debian_version ]; then
    curl -sL 'https://apt.netmaker.org/gpg.key' | sudo tee /etc/apt/trusted.gpg.d/netclient.asc
    curl -sL 'https://apt.netmaker.org/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/netclient.list
    sudo apt update
    sudo apt install netclient
elif [ -f /etc/centos-release ]; then
    curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
    curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
    sudo rpm --import /tmp/gpg.key
    sudo dnf check-update
    sudo dnf install netclient
elif [ -f /etc/fedora-release ]; then
    curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
    curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
    sudo rpm --import /tmp/gpg.key
    sudo dnf check-update
    sudo dnf install netclient
elif [ -f /etc/redhat-release ]; then
    curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
    curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
    sudo rpm --import /tmp/gpg.key
    sudo dnf check-update(
    sudo dnf install netclient
elif [ -f /etc/arch-release ]; then
    yay -S netclient
else
	echo "OS not supported for automatic install"
    exit 1
fi

register_

if [ -z "${install_cmd}" ]; then
        echo "OS unsupported for automatic dependency install"
	exit 1
fi
)}

setup_nmctl() {(
    wget https://github.com/gravitl/netmaker/releases/download/v0.17.1/nmctl
    chmod +x nmctl
    ./nmctl context set default --endpoint="https://$SERVER_HTTP_HOST" --master_key="$MASTER_KEY"
    ./nmctl context use default
    RESP=$(./nmctl network list)
    if [[ $RESP == *"unauthorized"* ]]; then
        echo "Unable to properly configure NMCTL, exiting..."
        exit 1
    fi
)}

join_networks() {(

NODE_LEN=$(jq length nodejson.tmp)
HAS_INGRESS="no"
echo $NODE_LEN
if [ "$NODE_LEN" -gt 0 ]; then
    for i in $(seq 1 $NODE_LEN); do
        NUM=$(($i-1))
        echo "  joining network $(jq ".[$NUM].network" ./nodejson.tmp):"
        KEY_JSON=./nmctl keys create $(jq ".[$NUM].network" ./nodejson.tmp) 1
        KEY=$(echo $KEY_JSON | jq -r .accessstring)
        NAME=$(jq ".[$NUM].name" ./nodejson.tmp)
        netclient join -t $KEY --name=""
        echo "    network: $(jq ".[$NUM].network" ./nodejson.tmp)"
        echo "      name: $(jq ".[$NUM].name" ./nodejson.tmp)"
        echo "      private ipv4: $(jq ".[$NUM].address" ./nodejson.tmp)"
        echo "      private ipv6: $(jq ".[$NUM].address6" ./nodejson.tmp)"
        echo "      is egress: $(jq ".[$NUM].isegressgateway" ./nodejson.tmp)"
        if [[ $(jq ".[$NUM].isegressgateway" ./nodejson.tmp) == "yes" ]]; then
            echo "          egress range: $(jq ".[$NUM].egressgatewayranges" ./nodejson.tmp)"
        fi

        HOST_ID=$(yq e .host.id /etc/netclient/netclient.yml)
        # set as a default host
        
        # create an egress if necessary
        # create an ingress if necessary
        echo "      is ingress: $(jq ".[$NUM].isingressgateway" ./nodejson.tmp)"
        if [[ $(jq ".[$NUM].isingressgateway" ./nodejson.tmp) == "yes" ]]; then
            HAS_INGRESS="yes"
        fi
        echo "      is relay: $(jq ".[$NUM].isrelay" ./nodejson.tmp)"
        echo "      is failover: $(jq ".[$NUM].failover" ./nodejson.tmp)"
        echo "  ------------"
    done
    echo "=================="
else
    echo "no networks to join"
fi


)}

setup_netmaker() {( set -e


for i in 1 2 3 4 5 6
do
	echo "    waiting for server node to become available"
	wait_seconds 10
	curlresponse=$(curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/netmaker)
	SERVER_ID=$(jq -r '.[0].id' <<< ${curlresponse})
	echo "    Server ID: $SERVER_ID"
	if [ $SERVER_ID == "null" ]; then
		SERVER_ID=""
	fi
	if [[ "$i" -ge "6" && -z "$SERVER_ID" ]]; then
		echo "    Netmaker is having issues configuring itself, please investigate (docker logs netmaker)"
		echo "    Exiting..."
		exit 1
	elif [ -z "$SERVER_ID" ]; then
		echo "    server node not yet configured, retrying..."
	elif [[ ! -z "$SERVER_ID" ]]; then
		echo "    server node is now availble, continuing"
		break
	fi
done


if [[ ! -z "$SERVER_ID"  ]]; then
	curl -o /dev/null -s -X POST -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/nodes/netmaker/$SERVER_ID/createingress
fi 
)}

set +e

# setup_netmaker
# wait_seconds 2
test_connection
wait_seconds 2
setup_netclient
wait_seconds 2
join_networks

echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
echo "Visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"

# cp -f /etc/skel/.bashrc /root/.bashrc
