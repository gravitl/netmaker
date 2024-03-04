#!/bin/bash

CONFIG_FILE=netmaker.env
# location of nm-quick.sh (usually `/root`)
SCRIPT_DIR=$(dirname "$(realpath "$0")")
CONFIG_PATH="$SCRIPT_DIR/$CONFIG_FILE"
NM_QUICK_VERSION="0.1.1"
LATEST=$(curl -s https://api.github.com/repos/gravitl/netmaker/releases/latest | grep "tag_name" | cut -d : -f 2,3 | tr -d [:space:],\")

if [ $(id -u) -ne 0 ]; then
	echo "This script must be run as root"
	exit 1
fi
# increase the timeouts
export DOCKER_CLIENT_TIMEOUT=120
export COMPOSE_HTTP_TIMEOUT=120
unset INSTALL_TYPE
unset BUILD_TAG
unset IMAGE_TAG
unset NETMAKER_BASE_DOMAIN
unset UPGRADE_FLAG
# usage - displays usage instructions
usage() {
	echo "nm-quick.sh v$NM_QUICK_VERSION"
	echo "usage: ./nm-quick.sh [-c]"
	echo " -c  if specified, will install netmaker community version"
	echo " -u  if specified, will upgrade netmaker to pro version"
	echo " -d if specified, will downgrade netmaker to community version"
	exit 1
}



# print_logo - prints the netmaker logo
print_logo() {
	cat <<"EOF"
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


	
	BUILD_TAG=$LATEST
	IMAGE_TAG=$(sed 's/\//-/g' <<<"$BUILD_TAG")

	if [ -z "$INSTALL_TYPE" ]; then
		echo "-----------------------------------------------------"
		echo "Would you like to install Netmaker Community Edition (CE), or Netmaker Enterprise Edition (pro)?"
		echo "pro will require you to create an account at https://app.netmaker.io"
		echo "-----------------------------------------------------"
		select install_option in "Community Edition" "Enterprise Edition"; do
			case $REPLY in
			1)
				echo "installing Netmaker CE"
				INSTALL_TYPE="ce"
				break
				;;
			2)
				echo "installing Netmaker pro"
				INSTALL_TYPE="pro"
				break
				;;
			*) echo "invalid option $REPLY" ;;
			esac
		done
	fi
	echo "-----------Build Options-----------------------------"
	echo "   Pro or CE: $INSTALL_TYPE"
	echo "   Build Tag: $BUILD_TAG"
	echo "   Image Tag: $IMAGE_TAG"
	echo "   Installer: v$NM_QUICK_VERSION"
	echo "-----------------------------------------------------"

}

# install_yq - install yq if not present
install_yq() {
	if ! command -v yq &>/dev/null; then
		wget -qO /usr/bin/yq https://github.com/mikefarah/yq/releases/download/v4.31.1/yq_linux_$(dpkg --print-architecture)
		chmod +x /usr/bin/yq
	fi
	set +e
	if ! command -v yq &>/dev/null; then
		set -e
		wget -qO /usr/bin/yq https://github.com/mikefarah/yq/releases/download/v4.31.1/yq_linux_amd64
		chmod +x /usr/bin/yq
	fi
	set -e
	if ! command -v yq &>/dev/null; then
		echo "failed to install yq. Please install yq and try again."
		echo "https://github.com/mikefarah/yq/#install"
		exit 1
	fi
}

# setup_netclient - adds netclient to docker-compose
setup_netclient() {

	set +e
	if [ -x "$(command -v netclient)" ]; then
		netclient uninstall
	fi
	set -e

	wget -qO netclient https://github.com/gravitl/netclient/releases/download/$LATEST/netclient-linux-$ARCH
	chmod +x netclient
	./netclient install
	echo "Register token: $TOKEN"
	netclient register -t $TOKEN

	echo "waiting for netclient to become available"
	local found=false
	local file=/etc/netclient/nodes.yml
	for ((a = 1; a <= 90; a++)); do
		if [ -f "$file" ]; then
			found=true
			break
		fi
		sleep 1
	done

	if [ "$found" = false ]; then
		echo "Error - $file not present"
		exit 1
	fi
}

# configure_netclient - configures server's netclient as a default host and an ingress gateway
configure_netclient() {

	NODE_ID=$(sudo cat /etc/netclient/nodes.yml | yq -r .netmaker.commonnode.id)
	if [ "$NODE_ID" = "" ] || [ "$NODE_ID" = "null" ]; then
		echo "Error obtaining NODE_ID for the new network"
		exit 1
	fi
	echo "register complete. New node ID: $NODE_ID"
	HOST_ID=$(sudo cat /etc/netclient/netclient.yml | yq -r .host.id)
	if [ "$HOST_ID" = "" ] || [ "$HOST_ID" = "null" ]; then
		echo "Error obtaining HOST_ID for the new network"
		exit 1
	fi
	echo "making host a default"
	echo "Host ID: $HOST_ID"
	# set as a default host
	set +e
	nmctl host update $HOST_ID --default
	sleep 5
	nmctl node create_ingress netmaker $NODE_ID
	#setup failOver
	sleep 5
	curl --location --request POST "https://api.${NETMAKER_BASE_DOMAIN}/api/v1/node/${NODE_ID}/failover" --header "Authorization: Bearer ${MASTER_KEY}"
	set -e
}

# setup_nmctl - pulls nmctl and makes it executable
setup_nmctl() {

	local URL="https://github.com/gravitl/netmaker/releases/download/$LATEST/nmctl-linux-$ARCH"
	echo "Downloading nmctl..."
	wget -qO /usr/bin/nmctl "$URL"

	if [ ! -f /usr/bin/nmctl ]; then
		echo "Error downloading nmctl from '$URL'"
		exit 1
	fi

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
wait_seconds() { (
	for ((a = 1; a <= $1; a++)); do
		echo ". . ."
		sleep 1
	done
); }

# confirm - get user input to confirm that they want to perform the next step
confirm() { (
	while true; do
		read -p 'Does everything look right? [y/n]: ' yn
		case $yn in
		[Yy]*)
			override="true"
			break
			;;
		[Nn]*)
			echo "exiting..."
			exit 1
			# TODO start from the beginning instead
			;;
		*) echo "Please answer yes or no." ;;
		esac
	done
) }


save_config() { (
	echo "Saving the config to $CONFIG_PATH"
	touch "$CONFIG_PATH"
	if [ -n "$EMAIL" ]; then 
		save_config_item NM_EMAIL "$EMAIL"
	fi
	if [ -n "$NETMAKER_BASE_DOMAIN" ]; then
		save_config_item NM_DOMAIN "$NETMAKER_BASE_DOMAIN"
	fi
	save_config_item UI_IMAGE_TAG "$IMAGE_TAG"
	# version-specific entries
	if [ "$INSTALL_TYPE" = "pro" ]; then
		save_config_item NETMAKER_TENANT_ID "$TENANT_ID"
		save_config_item LICENSE_KEY "$LICENSE_KEY"
		if [ "$UPGRADE_FLAG" = "yes" ];then
			save_config_item METRICS_EXPORTER "on"
			save_config_item PROMETHEUS "on"
		fi
		save_config_item SERVER_IMAGE_TAG "$IMAGE_TAG-ee"
	else
		save_config_item METRICS_EXPORTER "off"
		save_config_item PROMETHEUS "off"
		save_config_item SERVER_IMAGE_TAG "$IMAGE_TAG"
	fi
	# copy entries from the previous config
	local toCopy=("SERVER_HOST" "MASTER_KEY" "MQ_USERNAME" "MQ_PASSWORD"
		"INSTALL_TYPE" "NODE_ID" "DNS_MODE" "NETCLIENT_AUTO_UPDATE" "API_PORT"
		"CORS_ALLOWED_ORIGIN" "DISPLAY_KEYS" "DATABASE" "SERVER_BROKER_ENDPOINT" "VERBOSITY"
		"DEBUG_MODE"  "REST_BACKEND" "DISABLE_REMOTE_IP_CHECK" "TELEMETRY" "AUTH_PROVIDER" "CLIENT_ID" "CLIENT_SECRET"
		"FRONTEND_URL" "AZURE_TENANT" "OIDC_ISSUER" "EXPORTER_API_PORT" "JWT_VALIDITY_DURATION" "RAC_AUTO_DISABLE" "CACHING_ENABLED")
	for name in "${toCopy[@]}"; do
		save_config_item $name "${!name}"
	done
	# preserve debug entries
	if test -n "$NM_SKIP_BUILD"; then
		save_config_item NM_SKIP_BUILD "$NM_SKIP_BUILD"
	fi
	if test -n "$NM_SKIP_CLONE"; then
		save_config_item NM_SKIP_CLONE "$NM_SKIP_CLONE"
	fi
	if test -n "$NM_SKIP_DEPS"; then
		save_config_item NM_SKIP_DEPS "$NM_SKIP_DEPS"
	fi
); }

save_config_item() { (
	local NAME="$1"
	local VALUE="$2"
	#echo "$NAME=$VALUE"
	if test -z "$VALUE"; then
		# load the default for empty values
		VALUE=$(awk -F'=' "/^$NAME/ { print \$2}"  "$SCRIPT_DIR/netmaker.default.env")
		# trim quotes for docker
		VALUE=$(echo "$VALUE" | sed -E "s|^(['\"])(.*)\1$|\2|g")
		#echo "Default for $NAME=$VALUE"
	fi
	# TODO single quote passwords
	if grep -q "^$NAME=" "$CONFIG_PATH"; then
		# TODO escape | in the value
		sed -i "s|$NAME=.*|$NAME=$VALUE|" "$CONFIG_PATH"
	else
		echo "$NAME=$VALUE" >>"$CONFIG_PATH"
	fi
); }



# install_dependencies - install necessary packages to run netmaker
install_dependencies() {

	if test -n "$NM_SKIP_DEPS"; then
		return
	fi

	echo "checking dependencies..."

	OS=$(uname)
	if [ -f /etc/debian_version ]; then
		dependencies="git wireguard wireguard-tools dnsutils jq docker.io docker-compose grep gawk"
		update_cmd='apt update'
		install_cmd='apt-get install -y'
	elif [ -f /etc/alpine-release ]; then
		dependencies="git wireguard jq docker.io docker-compose grep gawk"
		update_cmd='apk update'
		install_cmd='apk --update add'
	elif [ -f /etc/centos-release ]; then
		dependencies="git wireguard jq bind-utils docker.io docker-compose grep gawk"
		update_cmd='yum update'
		install_cmd='yum install -y'
	elif [ -f /etc/fedora-release ]; then
		dependencies="git wireguard bind-utils jq docker.io docker-compose grep gawk"
		update_cmd='dnf update'
		install_cmd='dnf install -y'
	elif [ -f /etc/redhat-release ]; then
		dependencies="git wireguard jq docker.io bind-utils docker-compose grep gawk"
		update_cmd='yum update'
		install_cmd='yum install -y'
	elif [ -f /etc/arch-release ]; then
		dependencies="git wireguard-tools dnsutils jq docker.io docker-compose grep gawk"
		update_cmd='pacman -Sy'
		install_cmd='pacman -S --noconfirm'
	elif [ "${OS}" = "FreeBSD" ]; then
		dependencies="git wireguard wget jq docker.io docker-compose grep gawk"
		update_cmd='pkg update'
		install_cmd='pkg install -y'
	else
		install_cmd=''
	fi

	if [ -z "${install_cmd}" ]; then
		echo "OS unsupported for automatic dependency install"
		# TODO shouldnt exit, check if deps available, if not
		#  ask the user to install manually and continue when ready
		exit 1
	fi
	# TODO add other supported architectures
	ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
    	ARCH=amd64
    elif [ "$ARCH" = "aarch64" ]; then
    	ARCH=arm64
    else
    	echo "Unsupported architechure"
    	# exit 1
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
	if [ "$NETMAKER_BASE_DOMAIN" = "" ]; then
		NETMAKER_BASE_DOMAIN=nm.$(echo $IP_ADDR | tr . -).nip.io
	fi
	SERVER_HOST=$IP_ADDR
	if test -z "$MASTER_KEY"; then
		MASTER_KEY=$(
			tr -dc A-Za-z0-9 </dev/urandom | head -c 30
			echo ''
		)
	fi
	DOMAIN_TYPE=""
	echo "-----------------------------------------------------"
	echo "Would you like to use your own domain for netmaker, or an auto-generated domain?"
	echo "To use your own domain, add a Wildcard DNS record (e.x: *.netmaker.example.com) pointing to $SERVER_HOST"
	echo "IMPORTANT: Due to the high volume of requests, the auto-generated domain has been rate-limited by the certificate provider."
	echo "For this reason, we STRONGLY RECOMMEND using your own domain. Using the auto-generated domain may lead to a failed installation due to rate limiting."
	echo "-----------------------------------------------------"

	select domain_option in "Auto Generated ($NETMAKER_BASE_DOMAIN)" "Custom Domain (e.x: netmaker.example.com)"; do
		case $REPLY in
		1)
			echo "using $NETMAKER_BASE_DOMAIN for base domain"
			DOMAIN_TYPE="auto"
			break
			;;
		2)
			read -p "Enter Custom Domain (make sure  *.domain points to $SERVER_HOST first): " domain
			NETMAKER_BASE_DOMAIN=$domain
			echo "using $NETMAKER_BASE_DOMAIN"
			DOMAIN_TYPE="custom"
			break
			;;
		*) echo "invalid option $REPLY" ;;
		esac
	done

	wait_seconds 2

	echo "-----------------------------------------------------"
	echo "The following subdomains will be used:"
	echo "          dashboard.$NETMAKER_BASE_DOMAIN"
	echo "                api.$NETMAKER_BASE_DOMAIN"
	echo "             broker.$NETMAKER_BASE_DOMAIN"

	if [ "$UPGRADE_FLAG" = "yes" ]; then
		echo "         prometheus.$NETMAKER_BASE_DOMAIN"
		echo "  netmaker-exporter.$NETMAKER_BASE_DOMAIN"
		echo "            grafana.$NETMAKER_BASE_DOMAIN"
	fi

	echo "-----------------------------------------------------"

	if [[ "$DOMAIN_TYPE" == "custom" ]]; then
		echo "before continuing, confirm DNS is configured correctly, with records pointing to $SERVER_HOST"
		confirm
	fi

	wait_seconds 1

	unset GET_EMAIL
	while [ -z "$GET_EMAIL" ]; do
		read -p "Email Address for Domain Registration: " GET_EMAIL
	done
	EMAIL="$GET_EMAIL"

	wait_seconds 1

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

	if test -z "$MQ_PASSWORD"; then
		MQ_PASSWORD=$(
			tr -dc A-Za-z0-9 </dev/urandom | head -c 30
			echo ''
		)
	fi


	select domain_option in "Auto Generated / Config Password" "Input Your Own Password"; do
		case $REPLY in
		1)
			echo "using random password for mq"
			break
			;;
		2)
			while true; do
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
		*) echo "invalid option $REPLY" ;;
		esac
	done
	

	wait_seconds 2

	echo "-----------------------------------------------------------------"
	echo "                SETUP ARGUMENTS"
	echo "-----------------------------------------------------------------"
	echo "        domain: $NETMAKER_BASE_DOMAIN"
	echo "         email: $EMAIL"
	echo "     public ip: $SERVER_HOST"
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

	
	local BASE_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG"
	local COMPOSE_URL="$BASE_URL/compose/docker-compose.yml"
	local CADDY_URL="$BASE_URL/docker/Caddyfile"
	if [ "$INSTALL_TYPE" = "pro" ]; then
		local COMPOSE_OVERRIDE_URL="$BASE_URL/compose/docker-compose.pro.yml"
		local CADDY_URL="$BASE_URL/docker/Caddyfile-pro"
	fi
	wget -qO "$SCRIPT_DIR"/docker-compose.yml $COMPOSE_URL
	if [ "$UPGRADE_FLAG" = "yes" ]; then
		wget -qO "$SCRIPT_DIR"/docker-compose.override.yml $COMPOSE_OVERRIDE_URL
	elif [ -a "$SCRIPT_DIR"/docker-compose.override.yml ]; then
		rm -f "$SCRIPT_DIR"/docker-compose.override.yml
	fi
	wget -qO "$SCRIPT_DIR"/Caddyfile "$CADDY_URL"
	wget -qO "$SCRIPT_DIR"/netmaker.default.env "$BASE_URL/scripts/netmaker.default.env"
	wget -qO "$SCRIPT_DIR"/mosquitto.conf "$BASE_URL/docker/mosquitto.conf"
	wget -qO "$SCRIPT_DIR"/wait.sh "$BASE_URL/docker/wait.sh"

	chmod +x "$SCRIPT_DIR"/wait.sh
	mkdir -p /etc/netmaker

	# link .env to the user config
	ln -fs "$SCRIPT_DIR/netmaker.env" "$SCRIPT_DIR/.env"
	save_config

	echo "Starting containers..."

	# start docker and rebuild containers / networks
	cd "${SCRIPT_DIR}"
	docker-compose up -d --force-recreate
	cd -
	wait_seconds 2

}

# test_connection - tests to make sure Caddy has proper SSL certs
test_connection() {

	echo "Testing Caddy setup (please be patient, this may take 1-2 minutes)"
	for i in 1 2 3 4 5 6 7 8; do
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
			secs=$(($i * 5 + 10))
			echo "    Issue establishing connection...retrying in $secs seconds..."
		fi
		sleep $secs
	done

}

# setup_mesh - sets up a default mesh network on the server
setup_mesh() {

	wait_seconds 5

	local networkCount=$(nmctl network list -o json | jq '. | length')

	# add a network if none present
	if [ "$networkCount" -lt 1 ]; then
		echo "Creating netmaker network (10.101.0.0/16)"

		# TODO causes "Error Status: 400 Response: {"Code":400,"Message":"could not find any records"}"
		nmctl network create --name netmaker --ipv4_addr 10.101.0.0/16

		wait_seconds 5
	fi

	echo "Obtaining a netmaker enrollment key..."

	local tokenJson=$(nmctl enrollment_key create --tags netmaker --unlimited --networks netmaker)
	TOKEN=$(jq -r '.token' <<<${tokenJson})
	if test -z "$TOKEN"; then
		echo "Error creating an enrollment key"
		exit 1
	else
		echo "Enrollment key ready"
	fi

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

cleanup() {
	# remove the existing netclient's instance from the existing network
	if ! command -v netclient >/dev/null 2>&1; then
		return
	fi
	if command -v nmctl >/dev/null 2>&1; then
		local node_id=$(netclient list | jq '.[0].node_id' 2>/dev/null)
		# trim doublequotes
		node_id="${node_id//\"/}"
		if test -n "$node_id"; then
			echo "De-registering the existing netclient..."
			nmctl node delete netmaker $node_id >/dev/null 2>&1
		fi
	fi

	stop_services
}

stop_services(){
	echo "Stopping all containers, this will take a while please wait..."
	local containers=("mq" "netmaker-ui" "coredns" "turn" "caddy" "netmaker" "netmaker-exporter" "prometheus" "grafana")
	for name in "${containers[@]}"; do
		local running=$(docker ps | grep -w "$name")
		local exists=$(docker ps -a | grep -w "$name")
		if test -n "$running"; then
			docker stop "$name" 1>/dev/null
		fi
		if test -n "$exists"; then
			docker rm "$name" 1>/dev/null
		fi
	done
}

upgrade() {
	print_logo
	unset IMAGE_TAG
	unset BUILD_TAG
	IMAGE_TAG=$UI_IMAGE_TAG
	BUILD_TAG=$UI_IMAGE_TAG
	echo "-----------------------------------------------------"
	echo "Provide Details for pro installation:"
	echo "    1. Log into https://app.netmaker.io"
	echo "    2. follow instructions to get a license at: https://docs.netmaker.io/pro/pro-setup.html"
	echo "    3. Retrieve License and Tenant ID"
	echo "-----------------------------------------------------"
	unset LICENSE_KEY
	while [ -z "$LICENSE_KEY" ]; do
		read -p "License Key: " LICENSE_KEY
	done
	unset TENANT_ID
	while [ -z ${TENANT_ID} ]; do
		read -p "Tenant ID: " TENANT_ID
	done
	save_config
	# start docker and rebuild containers / networks
	stop_services
	install_netmaker
}

downgrade () {
	print_logo
	unset IMAGE_TAG
	unset BUILD_TAG
	IMAGE_TAG=$UI_IMAGE_TAG
	BUILD_TAG=$UI_IMAGE_TAG
	save_config
	if [ -a "$SCRIPT_DIR"/docker-compose.override.yml ]; then
		rm -f "$SCRIPT_DIR"/docker-compose.override.yml
	fi
	# start docker and rebuild containers / networks
	stop_services
	install_netmaker
}


main (){

	# read the config
	if [ -f "$CONFIG_PATH" ]; then
		echo "Using config: $CONFIG_PATH"
		source "$CONFIG_PATH"
	fi

	INSTALL_TYPE="pro"
	while getopts :cudv flag; do
	case "${flag}" in
	c)
		INSTALL_TYPE="ce"
		;;
	u)
		echo "upgrading to pro version..."
		INSTALL_TYPE="pro"
		UPGRADE_FLAG="yes"
		upgrade
		exit 0
		;;
	d) 
		echo "downgrading to community version..."
		INSTALL_TYPE="ce"
		downgrade
		exit 0
		;;
	v)
		usage
		exit 0
		;;
	esac
done

	# 1. print netmaker logo
	print_logo

	# 2. setup the build instructions
	set_buildinfo
	set +e
	# 3. install necessary packages
	install_dependencies

	# 4. install yq if necessary
	install_yq

	set -e

	# 6. get user input for variables
	set_install_vars

	set +e
	cleanup
	set -e

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
}

main "${@}"