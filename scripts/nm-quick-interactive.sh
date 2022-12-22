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

if [ -z "$1" ]; then
	echo "-----------------------------------------------------"
	echo "Would you like to install Netmaker Community Edition (CE), or Netmaker Enterprise Edition (EE)?"
	echo "EE will require you to create an account at https://dashboard.license.netmaker.io"
	echo "-----------------------------------------------------"
	select install_option in "Community Edition" "Enterprise Edition"; do
	case $REPLY in
		1)
		echo "installing Netmaker CE"
		INSTALL_TYPE="ce"
		break
		;;      
		2)
		echo "installing Netmaker EE"
		INSTALL_TYPE="ee"
		break
		;;
		*) echo "invalid option $REPLY";;
	esac
	done
elif [ "$1" = "ce" ]; then
	echo "installing Netmaker CE"
	INSTALL_TYPE="ce"
elif [ "$1" = "ee" ]; then
	echo "installing Netmaker EE"
	INSTALL_TYPE="ee"
else
	echo "install type invalid (options: 'ce, ee')"
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
	dependencies="wireguard wireguard-tools jq docker.io docker-compose"
	update_cmd='apt update'
	install_cmd='apt-get install -y'
elif [ -f /etc/alpine-release ]; then
	dependencies="wireguard jq docker.io docker-compose"
	update_cmd='apk update'
	install_cmd='apk --update add'
elif [ -f /etc/centos-release ]; then
	dependencies="wireguard jq docker.io docker-compose"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/fedora-release ]; then
	dependencies="wireguard jq docker.io docker-compose"
	update_cmd='dnf update'
	install_cmd='dnf install -y'
elif [ -f /etc/redhat-release ]; then
	dependencies="wireguard jq docker.io docker-compose"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/arch-release ]; then
    	dependecies="wireguard-tools jq docker.io docker-compose"
	update_cmd='pacman -Sy'
	install_cmd='pacman -S --noconfirm'
elif [ "${OS}" = "FreeBSD" ]; then
	dependencies="wireguard wget jq docker.io docker-compose"
	update_cmd='pkg update'
	install_cmd='pkg install -y'
elif [ -f /etc/turris-version ]; then
	dependencies="wireguard-tools bash jq docker.io docker-compose"
	OS="TurrisOS"
	update_cmd='opkg update'	
	install_cmd='opkg install'
elif [ -f /etc/openwrt_release ]; then
	dependencies="wireguard-tools bash jq docker.io docker-compose"
	OS="OpenWRT"
	update_cmd='opkg update'	
	install_cmd='opkg install'
else
	install_cmd=''
fi

if [ -z "${install_cmd}" ]; then
        echo "OS unsupported for automatic dependency install"
	exit 1
fi

set -- $dependencies

${update_cmd}

while [ -n "$1" ]; do
	if [ "${OS}" = "FreeBSD" ]; then
		is_installed=$(pkg check -d $1 | grep "Checking" | grep "done")
		if [ "$is_installed" != "" ]; then
			echo "  " $1 is installed
		else
			echo "  " $1 is not installed. Attempting install.
			${install_cmd} $1
			sleep 5
			is_installed=$(pkg check -d $1 | grep "Checking" | grep "done")
			if [ "$is_installed" != "" ]; then
				echo "  " $1 is installed
			elif [ -x "$(command -v $1)" ]; then
				echo "  " $1 is installed
			else
				echo "  " FAILED TO INSTALL $1
				echo "  " This may break functionality.
			fi
		fi	
	else
		if [ "${OS}" = "OpenWRT" ] || [ "${OS}" = "TurrisOS" ]; then
			is_installed=$(opkg list-installed $1 | grep $1)
		else
			is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
		fi
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
	fi
	shift
done

echo "-----------------------------------------------------"
echo "dependency check complete"
echo "-----------------------------------------------------"

wait_seconds 3

set -e

NETMAKER_BASE_DOMAIN=nm.$(curl -s ifconfig.me | tr . -).nip.io
COREDNS_IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p')
SERVER_PUBLIC_IP=$(curl -s ifconfig.me)
MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
MQ_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
DOMAIN_TYPE=""

echo "-----------------------------------------------------"
echo "Would you like to use your own domain for netmaker, or an auto-generated domain?"
echo "To use your own domain, add a Wildcard DNS record (e.x: *.netmaker.example.com) pointing to $SERVER_PUBLIC_IP"
echo "-----------------------------------------------------"
select domain_option in "Auto Generated ($NETMAKER_BASE_DOMAIN)" "Custom Domain (e.x: netmaker.example.com)"; do
  case $REPLY in
    1)
      echo "using $NETMAKER_BASE_DOMAIN for base domain"
      DOMAIN_TYPE="auto"
	  break
      ;;      
    2)
      read -p "Enter Custom Domain (make sure  *.domain points to $SERVER_PUBLIC_IP first): " domain
      NETMAKER_BASE_DOMAIN=$domain
      echo "using $NETMAKER_BASE_DOMAIN"
      DOMAIN_TYPE="custom"
      break
      ;;
    *) echo "invalid option $REPLY";;
  esac
done

wait_seconds 2

echo "-----------------------------------------------------"
echo "The following subdomains will be used:"
echo "          dashboard.$NETMAKER_BASE_DOMAIN"
echo "                api.$NETMAKER_BASE_DOMAIN"
echo "             broker.$NETMAKER_BASE_DOMAIN"

if [ "$INSTALL_TYPE" = "ee" ]; then
	echo "         prometheus.$NETMAKER_BASE_DOMAIN"
	echo "  netmaker-exporter.$NETMAKER_BASE_DOMAIN"
	echo "            grafana.$NETMAKER_BASE_DOMAIN"
fi

echo "-----------------------------------------------------"

if [[ "$DOMAIN_TYPE" == "custom" ]]; then
	echo "before continuing, confirm DNS is configured correctly, with records pointing to $SERVER_PUBLIC_IP"
	confirm
fi

wait_seconds 1

if [ "$INSTALL_TYPE" = "ee" ]; then

	echo "-----------------------------------------------------"
	echo "Provide Details for EE installation:"
	echo "    1. Log into https://dashboard.license.netmaker.io"
	echo "    2. Copy License Key Value: https://dashboard.license.netmaker.io/license-keys"
	echo "    3. Retrieve Account ID: https://dashboard.license.netmaker.io/user"
	echo "    4. note email address"
	echo "-----------------------------------------------------"
	unset LICENSE_KEY
	while [ -z "$LICENSE_KEY" ]; do
		read -p "License Key: " LICENSE_KEY
	done
	unset ACCOUNT_ID
	while [ -z ${ACCOUNT_ID} ]; do
		read -p "Account ID: " ACCOUNT_ID
	done

fi

unset GET_EMAIL
unset RAND_EMAIL
RAND_EMAIL="$(echo $RANDOM | md5sum  | head -c 16)@email.com"
read -p "Email Address for Domain Registration (click 'enter' to use $RAND_EMAIL): " GET_EMAIL
if [ -z "$GET_EMAIL" ]; then
  echo "using rand email"
  EMAIL="$RAND_EMAIL"
else
  EMAIL="$GET_EMAIL"
fi

wait_seconds 2

echo "-----------------------------------------------------------------"
echo "                SETUP ARGUMENTS"
echo "-----------------------------------------------------------------"
echo "        domain: $NETMAKER_BASE_DOMAIN"
echo "         email: $EMAIL"
echo "     public ip: $SERVER_PUBLIC_IP"
if [ "$INSTALL_TYPE" = "ee" ]; then
	echo "       license: $LICENSE_KEY"
	echo "    account id: $ACCOUNT_ID"
fi
echo "-----------------------------------------------------------------"
echo "Confirm Settings for Installation"
echo "- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -"

confirm


echo "-----------------------------------------------------------------"
echo "Beginning installation..."
echo "-----------------------------------------------------------------"

wait_seconds 3

echo "Pulling config files..."

COMPOSE_URL="https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.yml" 
CADDY_URL="https://raw.githubusercontent.com/gravitl/netmaker/master/docker/Caddyfile"
if [ "$INSTALL_TYPE" = "ee" ]; then
	COMPOSE_URL="https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.ee.yml" 
	CADDY_URL="https://raw.githubusercontent.com/gravitl/netmaker/master/docker/Caddyfile-EE"
fi

wget -O /root/docker-compose.yml $COMPOSE_URL && wget -O /root/mosquitto.conf https://raw.githubusercontent.com/gravitl/netmaker/master/docker/mosquitto.conf && wget -O /root/Caddyfile $CADDY_URL && wget -q -O /root/wait.sh https://raw.githubusercontent.com/gravitl/netmaker/master/docker/wait.sh && chmod +x /root/wait.sh

mkdir -p /etc/netmaker

echo "Setting docker-compose and Caddyfile..."

sed -i "s/SERVER_PUBLIC_IP/$SERVER_PUBLIC_IP/g" /root/docker-compose.yml
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/Caddyfile
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/docker-compose.yml
sed -i "s/REPLACE_MASTER_KEY/$MASTER_KEY/g" /root/docker-compose.yml
sed -i "s/YOUR_EMAIL/$EMAIL/g" /root/Caddyfile
sed -i "s/REPLACE_MQ_ADMIN_PASSWORD/$MQ_PASSWORD/g" /root/docker-compose.yml 
if [ "$INSTALL_TYPE" = "ee" ]; then
	sed -i "s~YOUR_LICENSE_KEY~$LICENSE_KEY~g" /root/docker-compose.yml
	sed -i "s/YOUR_ACCOUNT_ID/$ACCOUNT_ID/g" /root/docker-compose.yml
fi
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


setup_mesh() {( set -e

wait_seconds 15

echo "Creating netmaker network (10.101.0.0/16)"

curl -s -o /dev/null -d '{"addressrange":"10.101.0.0/16","netid":"netmaker"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/networks

wait_seconds 5

echo "Creating netmaker access key"

curlresponse=$(curl -s -d '{"uses":99999,"name":"netmaker-key"}' -H "Authorization: Bearer $MASTER_KEY" -H 'Content-Type: application/json' https://api.${NETMAKER_BASE_DOMAIN}/api/networks/netmaker/keys)
ACCESS_TOKEN=$(jq -r '.accessstring' <<< ${curlresponse})

wait_seconds 3

echo "Configuring netmaker server as ingress gateway"

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
test_connection

wait_seconds 3

setup_mesh

echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"
echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
echo "Visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
echo "-----------------------------------------------------------------"
echo "-----------------------------------------------------------------"

# cp -f /etc/skel/.bashrc /root/.bashrc
