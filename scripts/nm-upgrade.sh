#!/bin/bash

# make sure current version is 0.17.1 before continuing
check_version() {
  IMG_TAG=$(yq -r '.services.netmaker.image' docker-compose.yml)

  if [[ "$IMG_TAG" == *"v0.17.1"* ]]; then
      echo "version is $IMG_TAG"
  else
      echo "error, current version is $IMG_TAG"
      echo "please upgrade to v0.17.1 in order to use the upgrade script"
      exit 1
  fi
}

# wait a number of seconds, print a log
wait_seconds() {
  for ((a=1; a <= $1; a++))
  do
    echo ". . ."
    sleep 1
  done
}

# confirm a choice, or exit script
confirm() {
  while true; do
      read -p 'Does everything look right? [y/n]: ' yn
      case $yn in
          [Yy]* ) override="true"; break;;
          [Nn]* ) echo "exiting..."; exit 1;;
          * ) echo "Please answer yes or no.";;
      esac
  done
}

# install system dependencies necessary for script to run
install_dependencies() {
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

  echo "-----------------------------------------------------"
  echo "dependency install complete"
  echo "-----------------------------------------------------"
}

# retrieve server settings from existing compose file
collect_server_settings() {
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
      *) echo "invalid option $REPLY, choose 1 or 2";;
    esac
  done

  SERVER_HTTP_HOST=$(yq -r .services.netmaker.environment.SERVER_HTTP_HOST docker-compose.yml)
  echo "-----------------------------------------------------"
  echo "Is $SERVER_HTTP_HOST the correct api endpoint for your Netmaker installation?"
  echo "-----------------------------------------------------"
  select endpoint_option in "yes" "no (enter manually)"; do
    case $REPLY in
      1)
        echo "using $SERVER_HTTP_HOST for api endpoint"
      break
        ;;      
      2)
        read -p "Enter API Endpoint: " endpoint
        SERVER_HTTP_HOST=$endpoint
        echo "using $SERVER_HTTP_HOST"
        break
        ;;
      *) echo "invalid option $REPLY";;
    esac
  done

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

  SERVER_NAME=${BROKER_NAME#"broker."}
  echo "-----------------------------------------------------"
  echo "Is $SERVER_NAME the correct base domain for your installation?"
  echo "-----------------------------------------------------"
  select domain_option in "yes" "no (enter manually)"; do
    case $REPLY in
      1)
        echo "using $SERVER_NAME for domain"
      break
        ;;      
      2)
        read -p "Enter Server Domain: " broker
        SERVER_NAME=$server
        echo "using $SERVER_NAME"
        break
        ;;
      *) echo "invalid option $REPLY";;
    esac
  done

  STUN_NAME="stun.$SERVER_NAME"
  echo "-----------------------------------------------------"
  echo "Netmaker v0.18.0 requires a new DNS entry for $STUN_NAME."
  echo "Please confirm this is added to your DNS provider before continuing"
  echo "(note: this is not required if using an nip.io address"
  echo "-----------------------------------------------------"
  confirm
}

# get existing server node configuration
collect_node_settings() {
  curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://$SERVER_HTTP_HOST/api/nodes | jq -c '[ .[] | select(.isserver=="yes") ]' > nodejson.tmp
  NODE_LEN=$(jq length nodejson.tmp)
  HAS_INGRESS="no"
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
}

# set compose file with proper values
set_compose() {

  # DEV_TEMP - Temporary instructions for testing
  sed -i "s/v0.17.1/testing/g" /root/docker-compose.yml

  # RELEASE_REPLACE - Use this once release is ready
  #sed -i "s/v0.17.1/v0.18.0/g" /root/docker-compose.yml
  yq ".services.netmaker.environment.SERVER_NAME = \"$SERVER_NAME\"" -i /root/docker-compose.yml
  yq ".services.netmaker.environment += {\"BROKER_NAME\": \"$BROKER_NAME\"}" -i /root/docker-compose.yml  
  yq ".services.netmaker.environment += {\"STUN_NAME\": \"$STUN_NAME\"}" -i /root/docker-compose.yml  
  yq ".services.netmaker.environment += {\"STUN_PORT\": \"3478\"}" -i /root/docker-compose.yml  
}

start_containers() {
  docker-compose -f /root/docker-compose.yml up -d
}

# make sure caddy is working
test_caddy() {
  echo "Testing Caddy setup (please be patient, this may take 1-2 minutes)"
  for i in 1 2 3 4 5 6 7 8
  do
  curlresponse=$(curl -vIs https://${SERVER_HTTP_HOST} 2>&1)

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

setup_netclient() {

# DEV_TEMP - Temporary instructions for testing
wget https://fileserver.netmaker.org/testing/netclient
chmod +x netclient
./netclient install

# RELEASE_REPLACE - Use this once release is ready
# if [ -f /etc/debian_version ]; then
#     curl -sL 'https://apt.netmaker.org/gpg.key' | sudo tee /etc/apt/trusted.gpg.d/netclient.asc
#     curl -sL 'https://apt.netmaker.org/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/netclient.list
#     sudo apt update
#     sudo apt install netclient
# elif [ -f /etc/centos-release ]; then
#     curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
#     curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
#     sudo rpm --import /tmp/gpg.key
#     sudo dnf check-update
#     sudo dnf install netclient
# elif [ -f /etc/fedora-release ]; then
#     curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
#     curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
#     sudo rpm --import /tmp/gpg.key
#     sudo dnf check-update
#     sudo dnf install netclient
# elif [ -f /etc/redhat-release ]; then
#     curl -sL 'https://rpm.netmaker.org/gpg.key' | sudo tee /tmp/gpg.key
#     curl -sL 'https://rpm.netmaker.org/netclient-repo' | sudo tee /etc/yum.repos.d/netclient.repo
#     sudo rpm --import /tmp/gpg.key
#     sudo dnf check-update(
#     sudo dnf install netclient
# elif [ -f /etc/arch-release ]; then
#     yay -S netclient
# else
# 	echo "OS not supported for automatic install"
#     exit 1
# fi

if [ -z "${install_cmd}" ]; then
        echo "OS unsupported for automatic dependency install"
	exit 1
fi
}

setup_nmctl() {

  # DEV_TEMP - Temporary instructions for testing
  wget https://fileserver.netmaker.org/testing/nmctl
 
  # RELEASE_REPLACE - Use this once release is ready
  # wget https://github.com/gravitl/netmaker/releases/download/v0.17.1/nmctl
    chmod +x nmctl
    echo "using server $SERVER_HTTP_HOST"
    echo "using master key $MASTER_KEY"
    ./nmctl context set default --endpoint="https://$SERVER_HTTP_HOST" --master_key="$MASTER_KEY"
    ./nmctl context use default
    RESP=$(./nmctl network list)
    if [[ $RESP == *"unauthorized"* ]]; then
        echo "Unable to properly configure NMCTL, exiting..."
        exit 1
    fi
}

join_networks() {
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
}


cat << "EOF"
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
The Netmaker Upgrade Script: Upgrading to v0.18.0 so you don't have to!
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
EOF

set -e 

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

echo "...installing dependencies for script"
install_dependencies

echo "...confirming version is correct"
check_version

echo "...collecting necessary server settings"
collect_server_settings

echo "...setup nmctl"
setup_nmctl

echo "...retrieving current server node settings"
collect_node_settings

echo "...backing up docker compose to docker-compose.yml.backup"
cp /root/docker-compose.yml /root/docker-compose.yml.backup

echo "...setting docker-compose values"
set_compose

# DEV_TEMP
echo "end of test"
exit 0

echo "..starting containers"
start_containers

wait_seconds 3

echo "..testing Caddy proxy"
test_caddy

echo "..testing Netmaker health"
# TODO, implement health check
# netmaker_health_check

wait_seconds 2

echo "...setting up netclient"
setup_netclient
wait_seconds 2

echo "...join networks"
join_networks

echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
echo "Visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"