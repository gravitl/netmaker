#!/bin/bash

LATEST="v0.18.5"

print_logo() {(
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
)}

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

unset INSTALL_TYPE
unset BUILD_TYPE
unset BUILD_TAG
unset IMAGE_TAG
unset AUTO_BUILD

# usage - displays usage instructions
usage () {
    echo "usage: ./nm-quick.sh [-e] [-b buildtype] [-t tag] [-a auto]"
    echo "  -e      if specified, will install netmaker EE"
    echo "  -b      type of build; options:"
	echo "          \"version\" - will install a specific version of Netmaker using remote git and dockerhub"
	echo "          \"local\": - will install by cloning repo and and building images from git"
	echo "          \"branch\": - will install a specific branch using remote git and dockerhub"
    echo "  -t      tag of build; if buildtype=version, tag=version. If builtype=branch or builtype=local, tag=branch"
    echo "  -a      auto-build; skip prompts and use defaults, if none provided"
    echo "examples:"
	echo "          nm-quick.sh -e -b version -t $LATEST"
	echo "          nm-quick.sh -e -b local -t feature_v0.17.2_newfeature"	
	echo "          nm-quick.sh -e -b branch -t develop"
    exit 1
}

while getopts evab:t: flag
do
	case "${flag}" in
		e) 
			INSTALL_TYPE="ee"
			;;
		v) 
			usage
			exit 0
			;;
		a) 
			AUTO_BUILD="on"
			;;			
		b) 
			BUILD_TYPE=${OPTARG}
			if [[ ! "$BUILD_TYPE" =~ ^(version|local|branch)$ ]]; then
				echo "error: $BUILD_TYPE is invalid"
				echo "valid options: version, local, branch"
				usage
				exit 1
			fi
			;;
		t) 
			BUILD_TAG=${OPTARG}
			;;
	esac
done

# print_logo - prints the netmaker logo
print_logo() {
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
}

# set_buildinfo - sets the information based on script input for how the installation should be run
set_buildinfo() {

	if [ -z "$BUILD_TYPE" ]; then
		BUILD_TYPE="version"
		BUILD_TAG=$LATEST
	fi

	if [ -z "$BUILD_TAG" ] && [ "$BUILD_TYPE" = "version" ]; then
		BUILD_TAG=$LATEST
	fi

	if [ -z "$BUILD_TAG" ] && [ ! -z "$BUILD_TYPE" ]; then
		echo "error: must specify build tag when build type \"$BUILD_TYPE\" is specified"
		usage		
		exit 1
	fi

	IMAGE_TAG=$(sed 's/\//-/g' <<< "$BUILD_TAG")

	if [ "$1" = "ce" ]; then
		INSTALL_TYPE="ce"
	elif [ "$1" = "ee" ]; then
		INSTALL_TYPE="ee"
	fi

	if [ "$AUTO_BUILD" = "on" ] && [ -z "$INSTALL_TYPE" ]; then
		INSTALL_TYPE="ce"
	elif [ -z "$INSTALL_TYPE" ]; then
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
	fi
	echo "-----------Build Options-----------------------------"
	echo "    EE or CE: $INSTALL_TYPE";
	echo "  Build Type: $BUILD_TYPE";
	echo "   Build Tag: $BUILD_TAG";
	echo "   Image Tag: $IMAGE_TAG";
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

# setup_netclient - adds netclient to docker-compose
setup_netclient() {

	set +e
	netclient uninstall
	set -e

	wget -O netclient https://github.com/gravitl/netclient/releases/download/$LATEST/netclient_linux_amd64
	chmod +x netclient
	./netclient install
	netclient register -t $TOKEN

	echo "waiting for client to become available"
	wait_seconds 10 
}

# configure_netclient - configures server's netclient as a default host and an ingress gateway
configure_netclient() {

	NODE_ID=$(sudo cat /etc/netclient/nodes.yml | yq -r .netmaker.commonnode.id)
	echo "register complete. New node ID: $NODE_ID"
	HOST_ID=$(sudo cat /etc/netclient/netclient.yml | yq -r .host.id)
	echo "making host a default"
	echo "Host ID: $HOST_ID"
	# set as a default host
	set +e
	nmctl host update $HOST_ID --default
	sleep 5
	nmctl node create_ingress netmaker $NODE_ID
	set -e
}

# setup_nmctl - pulls nmctl and makes it executable
setup_nmctl() {

	wget -O /usr/bin/nmctl https://github.com/gravitl/netmaker/releases/download/$LATEST/nmctl_linux_amd64

    chmod +x /usr/bin/nmctl
    echo "using server api.$NETMAKER_BASE_DOMAIN"
    echo "using master key $MASTER_KEY"
    nmctl context set default --endpoint="https://api.$NETMAKER_BASE_DOMAIN" --master_key="$MASTER_KEY"
    nmctl context use default
    RESP=$(nmctl network list)
    if [[ $RESP == *"unauthorized"* ]]; then
        echo "Unable to properly configure NMCTL, exiting..."
        exit 1
    fi
}

# wait_seconds - wait for the specified period of time
wait_seconds() {(
  for ((a=1; a <= $1; a++))
  do
    echo ". . ."
    sleep 1
  done
)}

# confirm - get user input to confirm that they want to perform the next step
confirm() {(
  if [ "$AUTO_BUILD" = "on" ]; then
	return 0
  fi
  while true; do
      read -p 'Does everything look right? [y/n]: ' yn
      case $yn in
          [Yy]* ) override="true"; break;;
          [Nn]* ) echo "exiting..."; exit 1;;
          * ) echo "Please answer yes or no.";;
      esac
  done
)}

# local_install_setup - builds artifacts based on specified branch locally to use in install
local_install_setup() {(
	rm -rf netmaker-tmp
	mkdir netmaker-tmp
	cd netmaker-tmp
	git clone https://www.github.com/gravitl/netmaker
	cd netmaker
	git checkout $BUILD_TAG
	git pull origin $BUILD_TAG
	docker build --no-cache --build-arg version=$IMAGE_TAG -t gravitl/netmaker:$IMAGE_TAG .
	if [ "$INSTALL_TYPE" = "ee" ]; then
		cp compose/docker-compose.ee.yml /root/docker-compose.yml 
		cp docker/Caddyfile-EE /root/Caddyfile
	else
		cp compose/docker-compose.yml /root/docker-compose.yml 
		cp docker/Caddyfile /root/Caddyfile
	fi
	cp docker/mosquitto.conf /root/mosquitto.conf
	cp docker/wait.sh /root/wait.sh
	cd ../../
	rm -rf netmaker-tmp
)}

# install_dependencies - install necessary packages to run netmaker 
install_dependencies() {
	echo "checking dependencies..."

	OS=$(uname)
	if [ -f /etc/debian_version ]; then
		dependencies="git wireguard wireguard-tools dnsutils jq docker.io docker-compose"
		update_cmd='apt update'
		install_cmd='apt-get install -y'
	elif [ -f /etc/alpine-release ]; then
		dependencies="git wireguard jq docker.io docker-compose"
		update_cmd='apk update'
		install_cmd='apk --update add'
	elif [ -f /etc/centos-release ]; then
		dependencies="git wireguard jq bind-utils docker.io docker-compose"
		update_cmd='yum update'
		install_cmd='yum install -y'
	elif [ -f /etc/fedora-release ]; then
		dependencies="git wireguard bind-utils jq docker.io docker-compose"
		update_cmd='dnf update'
		install_cmd='dnf install -y'
	elif [ -f /etc/redhat-release ]; then
		dependencies="git wireguard jq docker.io bind-utils docker-compose"
		update_cmd='yum update'
		install_cmd='yum install -y'
	elif [ -f /etc/arch-release ]; then
			dependecies="git wireguard-tools dnsutils jq docker.io docker-compose"
		update_cmd='pacman -Sy'
		install_cmd='pacman -S --noconfirm'
	elif [ "${OS}" = "FreeBSD" ]; then
		dependencies="git wireguard wget jq docker.io docker-compose"
		update_cmd='pkg update'
		install_cmd='pkg install -y'
	elif [ -f /etc/turris-version ]; then
		dependencies="git wireguard-tools bash jq docker.io docker-compose"
		OS="TurrisOS"
		update_cmd='opkg update'	
		install_cmd='opkg install'
	elif [ -f /etc/openwrt_release ]; then
		dependencies="git wireguard-tools bash jq docker.io docker-compose"
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
} 
set -e

# set_install_vars - sets the variables that will be used throughout installation
set_install_vars() {

	IP_ADDR=$(dig -4 myip.opendns.com @resolver1.opendns.com +short)
	if [ "$IP_ADDR" = "" ]; then
		IP_ADDR=$(curl -s ifconfig.me)
	fi

	NETMAKER_BASE_DOMAIN=nm.$(echo $IP_ADDR | tr . -).nip.io
	COREDNS_IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p')
	SERVER_PUBLIC_IP=$IP_ADDR
	MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
	DOMAIN_TYPE=""
	echo "-----------------------------------------------------"
	echo "Would you like to use your own domain for netmaker, or an auto-generated domain?"
	echo "To use your own domain, add a Wildcard DNS record (e.x: *.netmaker.example.com) pointing to $SERVER_PUBLIC_IP"
	echo "-----------------------------------------------------"

	if [ "$AUTO_BUILD" = "on" ]; then
			DOMAIN_TYPE="auto"
	else
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
	fi

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
	if [ -z $AUTO_BUILD ]; then
		read -p "Email Address for Domain Registration (click 'enter' to use $RAND_EMAIL): " GET_EMAIL
	fi
	if [ -z "$GET_EMAIL" ]; then
	echo "using rand email"
	EMAIL="$RAND_EMAIL"
	else
	EMAIL="$GET_EMAIL"
	fi

	wait_seconds 1

	unset GET_MQ_USERNAME
	unset GET_MQ_PASSWORD
	unset CONFIRM_MQ_PASSWORD
	echo "Enter Credentials For MQ..."
	if [ -z $AUTO_BUILD ]; then
		read -p "MQ Username (click 'enter' to use 'netmaker'): " GET_MQ_USERNAME
	fi
	if [ -z "$GET_MQ_USERNAME" ]; then
	echo "using default username for mq"
	MQ_USERNAME="netmaker"
	else
	MQ_USERNAME="$GET_MQ_USERNAME"
	fi

	MQ_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')

	if [ -z $AUTO_BUILD ]; then  
		select domain_option in "Auto Generated Password" "Input Your Own Password"; do
			case $REPLY in
			1)
			echo "using random password for mq"
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

}

# install_netmaker - sets the config files and starts docker-compose
install_netmaker() {

	echo "-----------------------------------------------------------------"
	echo "Beginning installation..."
	echo "-----------------------------------------------------------------"

	wait_seconds 3

	echo "Pulling config files..."


	COMPOSE_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/compose/docker-compose.yml" 
	CADDY_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/docker/Caddyfile"
	if [ "$INSTALL_TYPE" = "ee" ]; then
		COMPOSE_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/compose/docker-compose.ee.yml" 
		CADDY_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/docker/Caddyfile-EE"
	fi
	if [ ! "$BUILD_TYPE" = "local" ]; then
		wget -O /root/docker-compose.yml $COMPOSE_URL && wget -O /root/mosquitto.conf https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/docker/mosquitto.conf && wget -O /root/Caddyfile $CADDY_URL
		wget -O /root/wait.sh https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG/docker/wait.sh
	fi

	chmod +x /root/wait.sh
	mkdir -p /etc/netmaker

	echo "Setting docker-compose and Caddyfile..."

	sed -i "s/SERVER_PUBLIC_IP/$SERVER_PUBLIC_IP/g" /root/docker-compose.yml
	sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/Caddyfile
	sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/docker-compose.yml
	sed -i "s/REPLACE_MASTER_KEY/$MASTER_KEY/g" /root/docker-compose.yml
	sed -i "s/YOUR_EMAIL/$EMAIL/g" /root/Caddyfile
	sed -i "s/REPLACE_MQ_PASSWORD/$MQ_PASSWORD/g" /root/docker-compose.yml
	sed -i "s/REPLACE_MQ_USERNAME/$MQ_USERNAME/g" /root/docker-compose.yml 
	if [ "$INSTALL_TYPE" = "ee" ]; then
		sed -i "s~YOUR_LICENSE_KEY~$LICENSE_KEY~g" /root/docker-compose.yml
		sed -i "s/YOUR_ACCOUNT_ID/$ACCOUNT_ID/g" /root/docker-compose.yml
	fi

	if [ "$BUILD_TYPE" = "version" ] && [ "$INSTALL_TYPE" = "ee" ]; then
		sed -i "s/REPLACE_SERVER_IMAGE_TAG/$IMAGE_TAG-ee/g" /root/docker-compose.yml
	else
		sed -i "s/REPLACE_SERVER_IMAGE_TAG/$IMAGE_TAG/g" /root/docker-compose.yml
	fi

	if [ "$BUILD_TYPE" = "local" ]; then
		sed -i "s/REPLACE_UI_IMAGE_TAG/$LATEST/g" /root/docker-compose.yml
	else
		sed -i "s/REPLACE_UI_IMAGE_TAG/$IMAGE_TAG/g" /root/docker-compose.yml
	fi

	echo "Starting containers..."

	docker-compose -f /root/docker-compose.yml up -d

	wait_seconds 2

}

# test_connection - tests to make sure Caddy has proper SSL certs
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

# setup_mesh - sets up a default mesh network on the server
setup_mesh() {

	wait_seconds 5

	echo "Creating netmaker network (10.101.0.0/16)"

	nmctl network create --name netmaker --ipv4_addr 10.101.0.0/16

	wait_seconds 5

	echo "Creating netmaker enrollment key"

	tokenJson=$(nmctl enrollment_key create --unlimited --networks netmaker)
	TOKEN=$(jq -r '.token' <<< ${tokenJson})

	wait_seconds 3

}

# print_success - prints a success message upon completion
print_success() {
	echo "-----------------------------------------------------------------"
	echo "-----------------------------------------------------------------"
	echo "Netmaker setup is now complete. You are ready to begin using Netmaker."
	echo "Visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
	echo "-----------------------------------------------------------------"
	echo "-----------------------------------------------------------------"
}

# 1. print netmaker logo
print_logo

# 2. setup the build instructions
set_buildinfo

set +e

# 3. install necessary packages
install_dependencies

# 4. install yq if necessary
install_yq

# 5. if running a local build, clone git and build artifacts
if [ "$BUILD_TYPE" = "local" ]; then
	local_install_setup
fi

set -e

# 6. get user input for variables
set_install_vars

# 7. get and set config files, startup docker-compose
install_netmaker

set +e

# 8. make sure Caddy certs are working
test_connection

# 9. install the netmaker CLI
setup_nmctl

# 10. create a default mesh network for netmaker
setup_mesh

set -e

# 11. add netclient to docker-compose and start it up
setup_netclient

# 12. make the netclient a default host and ingress gw
configure_netclient

# 13. print success message
print_success

# cp -f /etc/skel/.bashrc /root/.bashrc
