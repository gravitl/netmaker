#!/bin/bash

LATEST="v0.18.5"
INSTALL_PATH="/root"

trap restore_old_netmaker_instructions

# check_version - make sure current version is 0.17.1 before continuing
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

backup_v17_files() {
  mkdir $INSTALL_PATH/netmaker_0.17.1_backup
  cp $INSTALL_PATH/docker-compose.yml  $INSTALL_PATH/netmaker_0.17.1_backup/docker-compose.yml
  cp $INSTALL_PATH/Caddyfile $INSTALL_PATH/netmaker_0.17.1_backup/Caddyfile
  cp $INSTALL_PATH/mosquitto.conf $INSTALL_PATH/netmaker_0.17.1_backup/mosquitto.conf
  cp $INSTALL_PATH/wait.sh $INSTALL_PATH/netmaker_0.17.1_backup/wait.sh
}

backup_volumes() {
  cp -r /var/lib/docker/volumes/root_caddy_conf/ /var/lib/docker/volumes/root_caddy_conf-backup/
  cp -r /var/lib/docker/volumes/root_caddy_data/ /var/lib/docker/volumes/root_caddy_data-backup/
  cp -r /var/lib/docker/volumes/root_dnsconfig/ /var/lib/docker/volumes/root_dnsconfig-backup/
  cp -r /var/lib/docker/volumes/root_mosquitto_data/ /var/lib/docker/volumes/root_mosquitto_data-backup/
  cp -r /var/lib/docker/volumes/root_mosquitto_logs/ /var/lib/docker/volumes/root_mosquitto_logs-backup/
  cp -r /var/lib/docker/volumes/root_sqldata/ /var/lib/docker/volumes/root_sqldata-backup/
}

restore_old_netmaker_instructions() {
  echo "There was a problem with the installation. Your config files and volumes have been backed up."
  echo "To restore Netmaker back to v0.17.1, copy all the netmaker volume backups (caddy_conf-backup, caddy_data-backup, dnsconfig-backup, mosquitto_data-backup, mosquitto_logs-backup, and sqldata-backup) back to their regular names with out the -backup."
  echo "Your config files should be located in ${INSTALL_PATH}/netmaker_0.17.1_backup. Simply run cp ${INSTALL_PATH}/netmaker_0.17.1_backup/* . (include the .) and run docker-compose up -d."
  echo "Your netmaker should be back to v0.17.1"
}

get_install_path() {
  echo "-----------------------------------------------------"
  echo "Is your docker-compose located in $INSTALL_PATH ?"
  echo "-----------------------------------------------------"
  select install_option in "yes" "no (enter manually)"; do
    case $REPLY in
      1)
        echo "using $INSTALL_PATH for an installation path."
      break
        ;;      
      2)
        read -p "Enter path where your docker-compose is located: " install_path
        SERVER_HTTP_HOST=$install_path
        echo "using $INSTALL_PATH"
        break
        ;;
      *) echo "invalid option $REPLY";;
    esac
  done
}

# wait_seconds - wait a number of seconds, print a log
wait_seconds() {
  for ((a=1; a <= $1; a++))
  do
    echo ". . ."
    sleep 1
  done
}

# confirm - confirm a choice, or exit script
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

# install_dependencies - install system dependencies necessary for script to run
install_dependencies() {
  OS=$(uname)
  if [ -f /etc/debian_version ]; then
    dependencies="jq wireguard jq dnsutils docker-compose"
    update_cmd='apt update'
    install_cmd='apt install -y'
  elif [ -f /etc/centos-release ]; then
    dependencies="wireguard jq bind-utils docker-compose"
    update_cmd='yum update'
    install_cmd='yum install -y'
  elif [ -f /etc/fedora-release ]; then
    dependencies="wireguard jq bind-utils docker-compose"
    update_cmd='dnf update'
    install_cmd='dnf install -y'
  elif [ -f /etc/redhat-release ]; then
    dependencies="wireguard jq bind-utils docker-compose"
    update_cmd='yum update'
    install_cmd='yum install -y'
  elif [ -f /etc/arch-release ]; then
        dependencies="wireguard-tools jq dnsutils docker-compose netclient"
    update_cmd='pacman -Sy'
    install_cmd='pacman -S --noconfirm'
  else
    echo "OS not supported for automatic install"
      exit 1
  fi

  set -- $dependencies

  if command -v docker >/dev/null 2>&1 ; then
    echo "Docker found"
    echo "version: $(docker version)"
  else
    echo "Docker not found. adding to dependencies"
    dependencies+=" docker.io"
  fi

  ${update_cmd}

  set +e
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
  set -e
  
  echo "-----------------------------------------------------"
  echo "dependency install complete"
  echo "-----------------------------------------------------"
}

# install_yq - install yq if not present
install_yq() {
	if ! command -v yq &> /dev/null; then
		wget -O /usr/bin/yq https://github.com/mikefarah/yq/releases/download/v4.31.1/yq_linux_$(dpkg --print-architecture)
		chmod +x /usr/bin/yq
	fi
	set +e
	if ! command -v yq &> /dev/null; then
		set -e
		wget -O /usr/bin/yq https://github.com/mikefarah/yq/releases/download/v4.31.1/yq_linux_amd64
		chmod +x /usr/bin/yq
	fi
	set -e
	if ! command -v yq &> /dev/null; then
		echo "failed to install yq. Please install yq and try again."
		echo "https://github.com/mikefarah/yq/#install"
		exit 1
	fi	
}

# collect_server_settings - retrieve server settings from existing compose file
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

  STUN_DOMAIN="stun.$SERVER_NAME"
  echo "-----------------------------------------------------"
  echo "Netmaker v0.18 requires a new DNS entry for $STUN_DOMAIN."
  echo "Please confirm this is added to your DNS provider before continuing"
  echo "(note: this is not required if using an nip.io address)"
  echo "-----------------------------------------------------"
  confirm
}

# collect_node_settings - get existing server node configuration
collect_node_settings() {
  curl -s -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://$SERVER_HTTP_HOST/api/nodes | jq -c '[ .[] | select(.isserver=="yes") ]' > nodejson.tmp
  NODE_LEN=$(jq length nodejson.tmp)
  HAS_INGRESS="no"
  HAS_RELAY="no"
  if [ "$NODE_LEN" -gt 0 ]; then
      echo "===SERVER NODES==="
      for i in $(seq 1 $NODE_LEN); do
          NUM=$(($i-1))
          echo "  SERVER NODE $NUM:"
          echo "    network: $(jq -r ".[$NUM].network" ./nodejson.tmp)"
          echo "      name: $(jq -r ".[$NUM].name" ./nodejson.tmp)"
          echo "      private ipv4: $(jq -r ".[$NUM].address" ./nodejson.tmp)"
          echo "      private ipv6: $(jq -r ".[$NUM].address6" ./nodejson.tmp)"
          echo "      is egress: $(jq -r ".[$NUM].isegressgateway" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isegressgateway" ./nodejson.tmp) == "yes" ]]; then
              echo "          egress range: $(jq -r ".[$NUM].egressgatewayranges" ./nodejson.tmp)"
          fi
          echo "      is ingress: $(jq -r ".[$NUM].isingressgateway" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isingressgateway" ./nodejson.tmp) == "yes" ]]; then
              HAS_INGRESS="yes"
          fi
          echo "      is relay: $(jq -r ".[$NUM].isrelay" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isrelay" ./nodejson.tmp) == "yes" ]]; then
              HAS_RELAY="yes"
              echo "          relay addrs: $(jq -r ".[$NUM].relayaddrs" ./nodejson.tmp | tr -d '[]\n"[:space:]')"
          fi
          echo "      is failover: $(jq -r ".[$NUM].failover" ./nodejson.tmp)"
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
  if [[ $HAS_RELAY == "yes" ]]; then
      echo "WARNING: Your server contains a Relay. After upgrading, relay will be unset. Relay functionality has been moved to the 'host' level, and must be reconfigured once all machines are upgraded."
      confirm
  fi

}

# setup_caddy - updates Caddy with new info
setup_caddy() {

  echo "backing up Caddyfile to ${INSTALL_PATH}/Caddyfile.backup"
  cp $INSTALL_PATH/Caddyfile $INSTALL_PATH/Caddyfile.backup

  if grep -wq "acme.zerossl.com/v2/DV90" Caddyfile; then 
      echo "zerossl already set, continuing" 
  else 
    echo "editing Caddyfile"
    sed -i '0,/email/{s~email~acme_ca https://acme.zerossl.com/v2/DV90\n\t&~}' $INSTALL_PATH/Caddyfile
  fi

cat <<EOT >> $INSTALL_PATH/Caddyfile

# STUN
https://$STUN_DOMAIN {
  reverse_proxy netmaker:3478
}
EOT

}

# set_mq_credentials - sets mq credentials
set_mq_credentials() {

  unset GET_MQ_USERNAME
  unset GET_MQ_PASSWORD
  unset CONFIRM_MQ_PASSWORD
  echo "Enter Credentials For MQ..."
  read -p "MQ Username (click 'enter' to use 'netmaker'): " GET_MQ_USERNAME
  if [ -z "$GET_MQ_USERNAME" ]; then
    echo "using default username for mq"
    MQ_USERNAME="netmaker"
  else
    MQ_USERNAME="$GET_MQ_USERNAME"
  fi

  select domain_option in "Auto Generated Password" "Input Your Own Password"; do
    case $REPLY in
    1)
    echo "generating random password for mq"
    MQ_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
    break
    ;;      
      2)
    while true
      do
          echo "Enter your Password For MQ: " 
          read -s GET_MQ_PASSWORD
          echo "Enter your password again to confirm: "
          read -s CONFIRM_MQ_PASSWORD
          if [ ${GET_MQ_PASSWORD} != ${CONFIRM_MQ_PASSWORD} ]; then
              echo "wrong password entered, try again..."
              continue
          fi
      MQ_PASSWORD="$GET_MQ_PASSWORD"
          echo "MQ Password Saved Successfully!!"
          break
      done
        break
        ;;
      *) echo "invalid option $REPLY";;
    esac
  done
}

# set_compose - set compose file with proper values
set_compose() {

  set_mq_credentials

  echo "retrieving updated wait script and mosquitto conf"  
  rm $INSTALL_PATH/wait.sh
  rm $INSTALL_PATH/mosquitto.conf

  wget -O $INSTALL_PATH/wait.sh https://raw.githubusercontent.com/gravitl/netmaker/master/docker/wait.sh

  chmod +x $INSTALL_PATH/wait.sh

  wget -O $INSTALL_PATH/mosquitto.conf https://raw.githubusercontent.com/gravitl/netmaker/master/docker/mosquitto.conf

  chmod +x $INSTALL_PATH/mosquitto.conf

  # DEV_TEMP
  sed -i "s/v0.17.1/$LATEST/g" $INSTALL_PATH/docker-compose.yml

  STUN_PORT=3478

  # RELEASE_REPLACE - Use this once release is ready

  #sed -i "s/v0.17.1/v0.18.6/g" /root/docker-compose.yml
  yq ".services.netmaker.environment.SERVER_NAME = \"$SERVER_NAME\"" -i $INSTALL_PATH/docker-compose.yml
  yq ".services.netmaker.environment += {\"BROKER_ENDPOINT\": \"wss://$BROKER_NAME\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.environment += {\"SERVER_BROKER_ENDPOINT\": \"ws://mq:1883\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.environment += {\"STUN_LIST\": \"$STUN_DOMAIN:$STUN_PORT,stun1.netmaker.io:3478,stun2.netmaker.io:3478,stun1.l.google.com:19302,stun2.l.google.com:19302\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.environment += {\"MQ_PASSWORD\": \"$MQ_PASSWORD\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.environment += {\"MQ_USERNAME\": \"$MQ_USERNAME\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.environment += {\"STUN_PORT\": \"$STUN_PORT\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.netmaker.ports += \"3478:3478/udp\"" -i $INSTALL_PATH/docker-compose.yml

  yq ".services.mq.environment += {\"MQ_PASSWORD\": \"$MQ_PASSWORD\"}" -i $INSTALL_PATH/docker-compose.yml  
  yq ".services.mq.environment += {\"MQ_USERNAME\": \"$MQ_USERNAME\"}" -i $INSTALL_PATH/docker-compose.yml  


  #remove unnecessary ports
  yq eval 'del( .services.netmaker.ports[] | select(. == "51821*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.mq.ports[] | select(. == "8883*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.mq.ports[] | select(. == "1883*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.mq.expose[] | select(. == "8883*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.mq.expose[] | select(. == "1883*") )' -i $INSTALL_PATH/docker-compose.yml

  # delete unnecessary compose sections
  yq eval 'del(.services.netmaker.cap_add)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.sysctls)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.MQ_ADMIN_PASSWORD)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.MQ_HOST)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.MQ_PORT)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.MQ_SERVER_PORT)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.PORT_FORWARD_SERVICES)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.CLIENT_MODE)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.netmaker.environment.HOST_NETWORK)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del(.services.mq.environment.NETMAKER_SERVER_HOST)' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.netmaker.volumes[] | select(. == "mosquitto_data*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .services.mq.volumes[] | select(. == "mosquitto_data*") )' -i $INSTALL_PATH/docker-compose.yml
  yq eval 'del( .volumes.mosquitto_data )' -i $INSTALL_PATH/docker-compose.yml

}

# start_containers - run docker-compose up -d
start_containers() {
  docker-compose -f $INSTALL_PATH/docker-compose.yml up -d
}

# test_caddy - make sure caddy is working
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

# setup_netclient - adds netclient to docker-compose
setup_netclient() {

	set +e
	netclient uninstall
	HAS_APT=false
  set -e
  if command -v apt >/dev/null; then
    HAS_APT=true
  fi
  set +e

  if  [ "$HAS_APT" = "true" ]; then
    curl -sL 'https://apt.netmaker.org/gpg.key' | sudo tee /etc/apt/trusted.gpg.d/netclient.asc
    curl -sL 'https://apt.netmaker.org/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/netclient.list
    sudo apt update
    sudo apt install netclient
  else
     wget -O /tmp/netclient https://github.com/gravitl/netclient/releases/download/$LATEST/netclient_linux_amd64 

	  chmod +x /tmp/netclient
	  /tmp/netclient install
  fi

	netclient register -t $KEY

	echo "waiting for client to become available"
	wait_seconds 10 

}

# setup_nmctl - pulls nmctl and makes it executable
setup_nmctl() {

    wget -O nmctl https://github.com/gravitl/netmaker/releases/download/$LATEST/nmctl_linux_amd64
  
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

# join_networks - joins netclient into the networks using old settings
join_networks() {
  NODE_LEN=$(jq length nodejson.tmp)  
  if [ "$NODE_LEN" -gt 0 ]; then
      for i in $(seq 1 $NODE_LEN); do
          HAS_INGRESS="no"
          HAS_EGRESS="no"
          EGRESS_RANGES=""
          HAS_RELAY="no"
          RELAY_ADDRS=""
          HAS_FAILOVER="no"

          NUM=$(($i-1))
          NETWORK=$(jq -r ".[$NUM].network" ./nodejson.tmp)
          echo "  joining network $NETWORK with following settings. Please confirm:"
          echo "         network: $(jq -r ".[$NUM].network" ./nodejson.tmp)"
          echo "            name: $(jq -r ".[$NUM].name" ./nodejson.tmp)"
          echo "    private ipv4: $(jq -r ".[$NUM].address" ./nodejson.tmp)"
          echo "    private ipv6: $(jq -r ".[$NUM].address6" ./nodejson.tmp)"
          echo "       is egress: $(jq -r ".[$NUM].isegressgateway" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isegressgateway" ./nodejson.tmp) == "yes" ]]; then
              HAS_EGRESS="yes"
              echo "          egress ranges: $(jq -r ".[$NUM].egressgatewayranges" ./nodejson.tmp | tr -d '[]\n"[:space:]')"
              EGRESS_RANGES=$(jq -r ".[$NUM].egressgatewayranges" ./nodejson.tmp | tr -d '[]\n"[:space:]')
              EGRESS_RANGES=${EGRESS_RANGES//0.0.0.0\/0/0.0.0.0\/5,8.0.0.0\/7,11.0.0.0\/8,12.0.0.0\/6,16.0.0.0\/4,32.0.0.0\/3,64.0.0.0\/2,128.0.0.0\/3,160.0.0.0\/5,168.0.0.0\/6,172.0.0.0\/12,172.32.0.0\/11,172.64.0.0\/10,172.128.0.0\/9,173.0.0.0\/8,174.0.0.0\/7,176.0.0.0\/4,192.0.0.0\/9,192.128.0.0\/11,192.160.0.0\/13,192.169.0.0\/16,192.170.0.0\/15,192.172.0.0\/14,192.176.0.0\/12,192.192.0.0\/10,193.0.0.0\/8,194.0.0.0\/7,196.0.0.0\/6,200.0.0.0\/5,208.0.0.0\/4}
              EGRESS_RANGES=${EGRESS_RANGES//0::\/0/}
              EGRESS_RANGES=${EGRESS_RANGES//,,/,}
              EGRESS_RANGES=`echo $EGRESS_RANGES | sed 's/,*$//g'`
              EGRESS_RANGES=`echo $EGRESS_RANGES | sed 's/^,*//g'`

          fi
          echo "      is ingress: $(jq -r ".[$NUM].isingressgateway" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isingressgateway" ./nodejson.tmp) == "yes" ]]; then
              HAS_INGRESS="yes"
          fi
          echo "        is relay: $(jq -r ".[$NUM].isrelay" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].isrelay" ./nodejson.tmp) == "yes" ]]; then
              HAS_RELAY="yes"
              RELAY_ADDRS=$(jq -r ".[$NUM].relayaddrs" ./nodejson.tmp | tr -d '[]\n"[:space:]')
          fi
          echo "     is failover: $(jq -r ".[$NUM].failover" ./nodejson.tmp)"
          if [[ $(jq -r ".[$NUM].failover" ./nodejson.tmp) == "yes" ]]; then
              HAS_FAILOVER="yes"
          fi
          echo "  ------------"

          confirm

          if [[ $NUM -eq 0 ]]; then 
            echo "running command: ./nmctl enrollment_key create --uses 1 --networks $NETWORK"
          	KEY_JSON=$(./nmctl enrollment_key create --uses 1 --networks $NETWORK)
          	KEY=$(jq -r '.token' <<< ${KEY_JSON})

            echo "enrollment key created: $KEY"

            setup_netclient
          else
            HOST_ID=$(sudo cat /etc/netclient/netclient.yml | yq -r .host.id)
            ./nmctl host add_network $HOST_ID $NETWORK
          fi
          NAME=$(jq -r ".[$NUM].name" ./nodejson.tmp)
          ADDRESS=$(jq -r ".[$NUM].address" ./nodejson.tmp)
          ADDRESS6=$(jq -r ".[$NUM].address6" ./nodejson.tmp)

          echo "wait 10 seconds for netclient to be ready"
          sleep 10

          NODE_ID=$(sudo cat /etc/netclient/nodes.yml | yq -r .$NETWORK.commonnode.id)
          echo "join complete. New node ID: $NODE_ID"
          if [[ $NUM -eq 0 ]]; then
            HOST_ID=$(sudo cat /etc/netclient/netclient.yml | yq -r .host.id)
            echo "For first join, making host a default"
            echo "Host ID: $HOST_ID"
            # set as a default host
            set +e
            ./nmctl host update $HOST_ID --default
            sleep 2
            set -e            
          fi

          # create an egress if necessary
          if [[ $HAS_EGRESS == "yes" ]]; then
            echo "creating egress"            
            ./nmctl node create_egress $NETWORK $NODE_ID $EGRESS_RANGES
            sleep 2
          fi

          echo "HAS INGRESS: $HAS_INGRESS"
          # create an ingress if necessary
          if [[ $HAS_INGRESS == "yes" ]]; then
            if [[ $HAS_FAILOVER == "yes" ]]; then
              echo "creating ingress and failover..."
              ./nmctl node create_ingress $NETWORK $NODE_ID --failover
              sleep 2
            else
              echo "creating ingress..."
              ./nmctl node create_ingress $NETWORK $NODE_ID
              sleep 2
            fi
          fi

          # relay
          if [[ $HAS_RELAY == "yes" ]]; then
            echo "cannot recreate relay; relay functionality moved to host"
            # ./nmctl node create_relay $NETWORK $NODE_ID $RELAY_ADDRS
            # sleep 2
          fi

      done
      echo "=================="
  else
      echo "no networks to join"
  fi
}

cat << "EOF"
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
The Netmaker Upgrade Script: Upgrading to v0.18 so you don't have to!
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - 
EOF

set -e 

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

set +e

#backup volumes and v0.17.1 configs in case of failure.
backup_volumes
backup_v17_files

# get the installation path for docker-compose.yml and other config files
get_install_path

echo "...installing dependencies for script"
install_dependencies

echo "...installing yq if necessary"
install_yq

set -e

echo "...confirming version is correct"
check_version

echo "...collecting necessary server settings"
collect_server_settings

echo "...setup nmctl"
setup_nmctl

echo "...retrieving current server node settings"
collect_node_settings

echo "...backing up docker compose to docker-compose.yml.backup"
cp $INSTALL_PATH/docker-compose.yml $INSTALL_PATH/docker-compose.yml.backup

echo "...setting Caddyfile values"
setup_caddy

echo "...setting docker-compose values"
set_compose

echo "...starting containers"
start_containers

echo "...remove old mosquitto data"
# TODO - yq is not removing volume from docker compose
# docker volume rm root_mosquitto_data

wait_seconds 3

echo "..testing Caddy proxy"
test_caddy

echo "..testing Netmaker health"
# TODO, implement health check
# netmaker_health_check
# wait_seconds 2

wait_seconds 2

echo "...setup netclient"
join_networks

echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
echo "Visit dashboard.$SERVER_NAME to log in"
echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"