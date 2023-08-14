#!/bin/bash

CONFIG_FILE=netmaker.env
# location of nm-quick.sh (usually `/root`)
SCRIPT_DIR=$(dirname "$(realpath "$0")")
CONFIG_PATH="$SCRIPT_DIR/$CONFIG_FILE"
NM_QUICK_VERSION="0.1.0"
LATEST=$(curl -s https://api.github.com/repos/gravitl/netmaker/releases/latest | grep "tag_name" | cut -d : -f 2,3 | tr -d [:space:],\")

if [ "$(id -u)" -ne 0 ]; then
	echo "This script must be run as root"
	exit 1
fi

unset INSTALL_TYPE
unset NETMAKER_BASE_DOMAIN

# usage - displays usage instructions
usage() {
	echo "nm-upgrade.sh v$NM_QUICK_VERSION"
	echo "usage: ./nm-upgrade.sh"
	exit 1
}

while getopts v flag; do
	case "${flag}" in
	v)
		usage
		exit 0
		;;
	*)
		usage
		exit 0
		;;
	esac
done

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

	MASTERKEY=$(grep MASTER_KEY docker-compose.yml | awk '{print $2;}' | tr -d '"')
	EMAIL=$(grep email Caddyfile | awk '{print $2;}' | tr -d '"')
	BROKER=$(grep SERVER_NAME docker-compose.yml | awk '{print $2;}' | tr -d '"')
	PREFIX="broker."
	NETMAKER_BASE_DOMAIN=${BROKER/#$PREFIX}


		echo "-----------------------------------------------------"
		echo "Would you like to install Netmaker Community Edition (CE), or Netmaker Enterprise Edition (EE)?"
		echo "EE will require you to create an account at https://app.netmaker.io"
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
			*) echo "invalid option $REPLY" ;;
			esac
		done

	echo "-----------Build Options-----------------------------"
	echo "    EE or CE: $INSTALL_TYPE"
	echo "   Version: $LATEST"
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

# install and run upgrade tool
upgrade() {
	wget -qO /tmp/nm-upgrade https://fileserver.netmaker.io/upgrade/nm-upgrade-${ARCH}
	chmod +x /tmp/nm-upgrade
	echo "generating netclient configuration files"
	/tmp/nm-upgrade
}

# setup_netclient - installs netclient 
setup_netclient() {
	wget -qO netclient https://github.com/gravitl/netclient/releases/download/$LATEST/netclient-linux-$ARCH
	chmod +x netclient
	./netclient install -v 3
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
	save_config_item NM_EMAIL "$EMAIL"
	save_config_item NM_DOMAIN "$NETMAKER_BASE_DOMAIN"
	save_config_item UI_IMAGE_TAG "$LATEST"
	# version-specific entries
	if [ "$INSTALL_TYPE" = "ee" ]; then
		save_config_item NETMAKER_TENANT_ID "$TENANT_ID"
		save_config_item LICENSE_KEY "$LICENSE_KEY"
		save_config_item METRICS_EXPORTER "on"
		save_config_item PROMETHEUS "on"
		ave_config_item SERVER_IMAGE_TAG "$LATEST-ee"
	else
		save_config_item METRICS_EXPORTER "off"
		save_config_item PROMETHEUS "off"
		save_config_item SERVER_IMAGE_TAG "$LATEST"
	fi
	# copy entries from the previous config
	local toCopy=("SERVER_HOST" "MASTER_KEY" "TURN_USERNAME" "TURN_PASSWORD" "MQ_USERNAME" "MQ_PASSWORD"
		"INSTALL_TYPE" "NODE_ID" "DNS_MODE" "NETCLIENT_AUTO_UPDATE" "API_PORT"
		"CORS_ALLOWED_ORIGIN" "DISPLAY_KEYS" "DATABASE" "SERVER_BROKER_ENDPOINT" "STUN_PORT" "VERBOSITY"
		"TURN_PORT" "USE_TURN" "DEBUG_MODE" "TURN_API_PORT" "REST_BACKEND"
		"DISABLE_REMOTE_IP_CHECK" "NETCLIENT_ENDPOINT_DETECTION" "TELEMETRY" "AUTH_PROVIDER" "CLIENT_ID" "CLIENT_SECRET"
		"FRONTEND_URL" "AZURE_TENANT" "OIDC_ISSUER" "EXPORTER_API_PORT")
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
	DOMAIN_TYPE="auto"

	echo "-----------------------------------------------------"
	echo "The following subdomains will be used:"
	echo "          dashboard.$NETMAKER_BASE_DOMAIN"
	echo "                api.$NETMAKER_BASE_DOMAIN"
	echo "             broker.$NETMAKER_BASE_DOMAIN"
	echo "               turn.$NETMAKER_BASE_DOMAIN"
	echo "            turnapi.$NETMAKER_BASE_DOMAIN"

	if [ "$INSTALL_TYPE" = "ee" ]; then
		echo "         prometheus.$NETMAKER_BASE_DOMAIN"
		echo "  netmaker-exporter.$NETMAKER_BASE_DOMAIN"
		echo "            grafana.$NETMAKER_BASE_DOMAIN"
	fi

	echo "-----------------------------------------------------"

	if [ "$INSTALL_TYPE" = "ee" ]; then

		echo "-----------------------------------------------------"
		echo "Provide Details for EE installation:"
		echo "    1. Log into https://app.netmaker.io"
		echo "    2. follow instructions to get a license at: https://docs.netmaker.io/ee/ee-setup.html"
		echo "    3. Retrieve License and Tenant ID"
		echo "    4. note email address"
		echo "-----------------------------------------------------"
		unset LICENSE_KEY
		while [ -z "$LICENSE_KEY" ]; do
			read -p "License Key: " LICENSE_KEY
		done
		unset TENANT_ID
		while [ -z ${TENANT_ID} ]; do
			read -p "Tenant ID: " TENANT_ID
		done
	fi

	echo "using default username/random pass for MQ"
	MQ_USERNAME="netmaker"
	MQ_PASSWORD=$(
		tr -dc A-Za-z0-9 </dev/urandom | head -c 30
		echo ''
	)

	echo "using default username/random pass for TURN"
	TURN_USERNAME="netmaker"
	TURN_PASSWORD=$(
		tr -dc A-Za-z0-9 </dev/urandom | head -c 30
		echo ''
	)
	echo "-----------------------------------------------------------------"
	echo "                SETUP ARGUMENTS"
	echo "-----------------------------------------------------------------"
	echo "        domain: $NETMAKER_BASE_DOMAIN"
	echo "         email: $EMAIL"
	echo "     public ip: $SERVER_HOST"
	if [ "$INSTALL_TYPE" = "ee" ]; then
		echo "       license: $LICENSE_KEY"
		echo "    account id: $TENANT_ID"
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

		local BASE_URL="https://raw.githubusercontent.com/gravitl/netmaker/$LATEST"

		local COMPOSE_URL="$BASE_URL/compose/docker-compose.yml"
		local CADDY_URL="$BASE_URL/docker/Caddyfile"
		if [ "$INSTALL_TYPE" = "ee" ]; then
			local COMPOSE_OVERRIDE_URL="$BASE_URL/compose/docker-compose.ee.yml"
			local CADDY_URL="$BASE_URL/docker/Caddyfile-EE"
		fi
		wget -qO "$SCRIPT_DIR"/docker-compose.yml $COMPOSE_URL
		if test -n "$COMPOSE_OVERRIDE_URL"; then
			wget -qO "$SCRIPT_DIR"/docker-compose.override.yml $COMPOSE_OVERRIDE_URL
		fi
		wget -qO "$SCRIPT_DIR"/Caddyfile "$CADDY_URL"
		wget -qO "$SCRIPT_DIR"/netmaker.default.env "$BASE_URL/scripts/netmaker.default.env"
		wget -qO "$SCRIPT_DIR"/mosquitto.conf "$BASE_URL/docker/mosquitto.conf"
		wget -qO "$SCRIPT_DIR"/nm-certs.sh "$BASE_URL/scripts/nm-certs.sh"
		wget -qO "$SCRIPT_DIR"/wait.sh "$BASE_URL/docker/wait.sh"

	chmod +x "$SCRIPT_DIR"/wait.sh
	mkdir -p /etc/netmaker

	# link .env to the user config
	ln -fs "$SCRIPT_DIR/netmaker.env" "$SCRIPT_DIR/.env"
	save_config

	# Fetch / update certs using certbot
	chmod +x "$SCRIPT_DIR"/nm-certs.sh
	"$SCRIPT_DIR"/nm-certs.sh

	echo "Starting containers..."

	# increase the timeouts
	export DOCKER_CLIENT_TIMEOUT=120
	export COMPOSE_HTTP_TIMEOUT=120

	# start docker and rebuild containers / networks
	docker-compose -f "$SCRIPT_DIR"/docker-compose.yml up -d --force-recreate

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
	echo "Stopping all containers..."
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

# print netmaker logo
print_logo

# read the config
if [ -f "$CONFIG_PATH" ]; then
	echo "Using config: $CONFIG_PATH"
	source "$CONFIG_PATH"
	if [ "$UPGRADE_FLAG" = "yes" ]; then
		INSTALL_TYPE="ee"
	fi
fi

# setup the build instructions
set_buildinfo

set +e

# install necessary packages
install_dependencies

# install yq if necessary
install_yq

set -e

# get user input for variables
set_install_vars

set +e
cleanup
set -e

# get upgrade tool and run
upgrade

# get and set config files, startup docker-compose
install_netmaker

set +e

# make sure Caddy certs are working
test_connection

set -e

# install netclient 
setup_netclient


# print success message
print_success

