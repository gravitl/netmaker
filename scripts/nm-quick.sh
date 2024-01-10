#!/bin/bash
set -eEuo pipefail

NM_QUICK_VERSION="0.1.2"
ARGV=("$@")

if [ "$(id -u)" -ne 0 ]; then
	echo "This script must be run as root" >&2
	exit 1
fi

configure() {
	# set to empty instead of swapping all calls to "${VAR:-""}"
	: "${NM_SKIP_BUILD:=''}"
	: "${NM_SKIP_DEPS:=''}"
	: "${NM_SKIP_CLIENT:=''}"
	: "${NM_SKIP_CLONE:=''}"

	CONFIG_FILENAME=netmaker.env
	# location of nm-quick.sh (usually `/root`)
	SCRIPT_DIR="${BASH_SOURCE[0]%/*}"

	XDG_DATA_HOME="${XDG_DATA_HOME:-"$HOME/.local/share"}"
	NM_QUICK_DATA_DIR="${NM_QUICK_DATA_DIR:-"$XDG_DATA_HOME/netmaker/nm-quick"}"
	if [ -f "${SCRIPT_DIR}/${CONFIG_FILENAME}" ]; then
		# backwards compatibility for old installs
		NM_QUICK_DATA_DIR="${SCRIPT_DIR}"
	fi

	DATA_DIR="${NM_QUICK_DATA_DIR}"
	CONFIG_PATH="$DATA_DIR/$CONFIG_FILENAME"
	info "Running in ${DATA_DIR}"
	mkdir -p "${DATA_DIR}/bin"
	pushd "${DATA_DIR}"

	export PATH="${DATA_DIR}/bin:${PATH}"

	LATEST="$(get_latest_version)"

	OS="$(uname)"
	# TODO add other supported architectures
	case "$(uname -m)" in
	x86_64) ARCH=amd64 ;;
	aarch64) ARCH=arm64 ;;
	*) info "Unsupported architecture" && exit 1 ;;
	esac

	# read arguments
	read_arguments "$@"

	# read the config
	load_config

	set_buildinfo
}

# usage - displays usage instructions
usage() {
	cat <<EOF
nm-quick.sh v${NM_QUICK_VERSION}
usage: ./nm-quick.sh [-e] [-b buildtype] [-t tag] [-a auto] [-d domain]
  -e      if specified, will install netmaker pro
  -b      type of build; options:
          \"version\" - will install a specific version of Netmaker using remote git and dockerhub
          \"local\": - will install by cloning repo and building images from git
          \"branch\": - will install a specific branch using remote git and dockerhub
  -t      tag of build; if buildtype=version, tag=version. If builtype=branch or builtype=local, tag=branch
  -a      auto-build; skip prompts and use defaults, if none provided
  -d      domain; if specified, will use this domain instead of auto-generating one
  -m      email; if specified, will use this email for certificate requests instead of auto-generating one
  -C      don't configure Netclient
examples:
          nm-quick.sh -e -b version -t $LATEST
          nm-quick.sh -e -b local -t feature_v0.17.2_newfeature
          nm-quick.sh -e -b branch -t develop
          nm-quick.sh -e -b version -t $LATEST -a -d example.com
EOF
}

info() {
	echo "$@" >&2
}

load_config() {
	if [ -f "$CONFIG_PATH" ]; then
		info "Using config: $CONFIG_PATH"
		# shellcheck disable=SC1090
		source "$CONFIG_PATH"
		if [ "${UPGRADE_FLAG:-}" = "yes" ]; then
			INSTALL_TYPE="pro"
		fi
	fi
}

read_arguments() {
	INSTALL_TYPE="ce"

	BUILD_TYPE=''
	BUILD_TAG=''
	IMAGE_TAG=''
	AUTO_BUILD=''
	NETMAKER_BASE_DOMAIN=''

	while getopts evabC:d:t:m: flag; do
		case "${flag}" in
		e)
			INSTALL_TYPE="pro"
			UPGRADE_FLAG="yes"
			;;
		v)
			usage
			exit 0
			;;
		a)
			AUTO_BUILD="on"
			;;
		b)
			BUILD_TYPE="${OPTARG}"
			if [[ ! "$BUILD_TYPE" =~ ^(version|local|branch)$ ]]; then
				info "error: $BUILD_TYPE is invalid"
				info "valid options: version, local, branch"
				usage
				exit 1
			fi
			;;
		t)
			BUILD_TAG="${OPTARG}"
			;;
		d)
			NETMAKER_BASE_DOMAIN="${OPTARG}"
			;;
		m)
			NM_EMAIL="${OPTARG}"
			;;
		C)
			NM_SKIP_CLIENT=1
			;;
		*)
			info "error: unknown flag ${flag}"
			usage
			exit 1
			;;
		esac
	done
}

get_latest() {
	wget -qO- https://api.github.com/repos/gravitl/netmaker/releases/latest | if command -v jq &>/dev/null; then
		jq -r .tag_name
	else
		grep "tag_name" | cut -d : -f 2,3 | tr -d '[:space:],"'
	fi
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

get_latest_version() {
	wget -qO- https://api.github.com/repos/gravitl/netmaker/releases/latest | if command -v jq &>/dev/null; then
		jq -r .tag_name
	else
		grep "tag_name" | cut -d : -f 2,3 | tr -d '[:space:],"'
	fi
}

# set_buildinfo - sets the information based on script input for how the installation should be run
set_buildinfo() {
	if [ -z "$BUILD_TYPE" ]; then
		BUILD_TYPE="version"
		BUILD_TAG="$LATEST"
	fi

	if [ -z "$BUILD_TAG" ] && [ "$BUILD_TYPE" = "version" ]; then
		BUILD_TAG="$LATEST"
	fi

	if [ -z "$BUILD_TAG" ] && [ -n "$BUILD_TYPE" ]; then
		info "error: must specify build tag when build type \"$BUILD_TYPE\" is specified"
		usage
		exit 1
	fi

	IMAGE_TAG=$(sed 's/\//-/g' <<<"$BUILD_TAG")

	if [ "$AUTO_BUILD" = "on" ] && [ -z "$INSTALL_TYPE" ]; then
		INSTALL_TYPE="ce"
	elif [ -z "$INSTALL_TYPE" ]; then
		info "-----------------------------------------------------"
		info "Would you like to install Netmaker Community Edition (CE), or Netmaker Enterprise Edition (pro)?"
		info "pro will require you to create an account at https://app.netmaker.io"
		info "-----------------------------------------------------"
		select _ in "Community Edition" "Enterprise Edition"; do
			case $REPLY in
			1)
				info "installing Netmaker CE"
				INSTALL_TYPE="ce"
				break
				;;
			2)
				info "installing Netmaker pro"
				INSTALL_TYPE="pro"
				break
				;;
			*) info "invalid option $REPLY" ;;
			esac
		done
	fi

	info "-----------Build Options-----------------------------"
	info "        Pro or CE: $INSTALL_TYPE"
	info "       Build Type: $BUILD_TYPE"
	info "        Build Tag: $BUILD_TAG"
	info "        Image Tag: $IMAGE_TAG"
	info "        Installer: v$NM_QUICK_VERSION"
	info " Operating System: $OS"
	info "     Architecture: $ARCH"
	info "      Install Dir: $DATA_DIR"
	info "-----------------------------------------------------"
}

setup_yq() {
	if ! command -v yq &>/dev/null; then
		wget -qO "${DATA_DIR}"/bin/yq https://github.com/mikefarah/yq/releases/download/v4.31.1/yq_linux_"$ARCH"
		chmod +x "${DATA_DIR}"/bin/yq
	fi
	if ! command -v yq &>/dev/null; then
		info "failed to install yq. Please install yq and try again."
		info "https://github.com/mikefarah/yq/#install"
		exit 1
	fi
}

# installs and runs yq on demand
yq() {
	if test -z "$HAS_YQ"; then
		HAS_YQ=1
		setup_yq >&2
	fi
	command yq "$@"
}

# setup_netclient - adds netclient to docker-compose
setup_netclient() {
	if [ -n "$NM_SKIP_CLIENT" ]; then
		info "Skipping setup_netclient() due to NM_SKIP_CLIENT"
		return
	fi

	netclient uninstall || :

	netclient install

	local token
	token="$(get_enrollment_key)"
	info "Register token: $token"
	netclient register -t "$token"

	info "waiting for netclient to become available"
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
		info "Error - $file not present"
		exit 1
	fi
}

netclient() {
	if test -z "$HAS_NETCLIENT"; then
		HAS_NETCLIENT=1
		wget -qO "${DATA_DIR}/bin/netclient" "https://github.com/gravitl/netclient/releases/download/$LATEST/netclient-linux-$ARCH"
		chmod +x "${DATA_DIR}/bin/netclient"
	fi
	command netclient "$@"
}

# configure_netclient - configures server's netclient as a default host and an ingress gateway
configure_netclient() {
	if [ -n "$NM_SKIP_CLIENT" ]; then
		info "Skipping configure_netclient() due to NM_SKIP_CLIENT"
		return
	fi

	NODE_ID=$(sudo cat /etc/netclient/nodes.yml | yq -r .netmaker.commonnode.id)
	if [ "$NODE_ID" = "" ] || [ "$NODE_ID" = "null" ]; then
		info "Error obtaining NODE_ID for the new network"
		exit 1
	fi
	info "register complete. New node ID: $NODE_ID"
	HOST_ID=$(sudo cat /etc/netclient/netclient.yml | yq -r .host.id)
	if [ "$HOST_ID" = "" ] || [ "$HOST_ID" = "null" ]; then
		info "Error obtaining HOST_ID for the new network"
		exit 1
	fi
	info "making host a default"
	info "Host ID: $HOST_ID"
	# set as a default host
	nmctl host update "$HOST_ID" --default || :
	sleep 5
	nmctl node create_ingress netmaker "$NODE_ID" || :
}

setup_nmctl() {
	local URL="https://github.com/gravitl/netmaker/releases/download/$LATEST/nmctl-linux-$ARCH"
	info "Downloading nmctl..."
	wget -qO "${DATA_DIR}/bin/nmctl" "$URL"

	if [ ! -f "${DATA_DIR}/bin/nmctl" ]; then
		info "Error downloading nmctl from '$URL'"
		exit 1
	fi
	chmod +x "${DATA_DIR}/bin/nmctl"

	info "using server api.$NETMAKER_BASE_DOMAIN"
	info "using master key Y"
	nmctl context set default --endpoint="https://api.$NETMAKER_BASE_DOMAIN" --master_key="$MASTER_KEY"
	nmctl context use default
	RESP=$(nmctl network list)
	if [[ $RESP == *"unauthorized"* ]]; then
		info "Unable to properly configure NMCTL, exiting..."
		exit 1
	fi
}

# nmctl - pulls and configures nmctl on demand
nmctl() {
	if test -z "$HAS_NMCTL"; then
		HAS_NMCTL=1
		setup_nmctl >&2
	fi
	command nmctl "$@"
}

# wait_seconds - wait for the specified period of time
wait_seconds() { (
	for ((a = 1; a <= $1; a++)); do
		info ". . ."
		sleep 1
	done
); }

# confirm - get user input to confirm that they want to perform the next step
confirm() { (
	if [ "$AUTO_BUILD" = "on" ]; then
		return 0
	fi
	while true; do
		read -r -p 'Does everything look right? [y/n]: ' yn
		case $yn in
		[Yy]*)
			break
			;;
		[Nn]*)
			info "starting from the beginning..."
			main "${ARGV[@]}"
			exit
			;;
		*) info "Please answer yes or no." ;;
		esac
	done
) }

save_config() { (
	info "Saving the config to $CONFIG_PATH"
	touch "$CONFIG_PATH"
	save_config_item NM_EMAIL "$EMAIL"
	save_config_item NM_DOMAIN "$NETMAKER_BASE_DOMAIN"
	save_config_item UI_IMAGE_TAG "$IMAGE_TAG"
	if [ "$BUILD_TYPE" = "local" ]; then
		save_config_item UI_IMAGE_TAG "$LATEST"
	else
		save_config_item UI_IMAGE_TAG "$IMAGE_TAG"
	fi
	# version-specific entries
	if [ "$INSTALL_TYPE" = "pro" ]; then
		save_config_item NETMAKER_TENANT_ID "$TENANT_ID"
		save_config_item LICENSE_KEY "$LICENSE_KEY"
		save_config_item METRICS_EXPORTER "on"
		save_config_item PROMETHEUS "on"
		if [ "$BUILD_TYPE" = "version" ]; then
			save_config_item SERVER_IMAGE_TAG "$IMAGE_TAG-ee"
		else
			save_config_item SERVER_IMAGE_TAG "$IMAGE_TAG"
		fi
	else
		save_config_item METRICS_EXPORTER "off"
		save_config_item PROMETHEUS "off"
		save_config_item SERVER_IMAGE_TAG "$IMAGE_TAG"
	fi

	# copy entries from the previous config
	local toCopyAlways=(
		API_PORT
		AUTH_PROVIDER
		AZURE_TENANT
		CLIENT_ID
		CLIENT_SECRET
		CORS_ALLOWED_ORIGIN
		DATABASE
		DEBUG_MODE
		DISABLE_REMOTE_IP_CHECK
		DISPLAY_KEYS
		DNS_MODE
		EXPORTER_API_PORT
		FRONTEND_URL
		INSTALL_TYPE
		JWT_VALIDITY_DURATION
		MASTER_KEY
		MQ_PASSWORD
		MQ_USERNAME
		NETCLIENT_AUTO_UPDATE
		NM_SKIP_BUILD
		NM_SKIP_CLIENT
		NM_SKIP_CLONE
		NM_SKIP_DEPS
		NODE_ID
		OIDC_ISSUER
		RAC_AUTO_DISABLE
		REST_BACKEND
		SERVER_BROKER_ENDPOINT
		SERVER_HOST
		TELEMETRY
		VERBOSITY
	)
	for name in "${toCopyAlways[@]}"; do
		save_config_item "$name"
	done
); }

save_config_item() {
	local NAME="$1"
	local VALUE="${2:"${!NAME:-""}"}"
	#info "$NAME=$VALUE"
	if test -z "$VALUE"; then
		# load the default for empty values
		VALUE="$(awk -F'=' "/^$NAME/ { print \$2}" "$DATA_DIR/netmaker.default.env")"
		# trim quotes for docker
		VALUE="$(info "$VALUE" | sed -E "s|^(['\"])(.*)\1$|\2|g")"
		#info "Default for $NAME=$VALUE"
	fi
	# escape | in the value
	VALUE="${VALUE//|/"\|"}"
	# escape single quotes
	VALUE="${VALUE//"'"/\'\"\'\"\'}"
	# single-quote the value
	VALUE="'${VALUE}'"
	if grep -q "^$NAME=" "$CONFIG_PATH"; then
		sed -i "s|$NAME=.*|$NAME=$VALUE|" "$CONFIG_PATH"
	else
		info "$NAME=$VALUE" >>"$CONFIG_PATH"
	fi
}

# local_install_setup - builds artifacts based on specified branch locally to use in install
local_install_setup() { (
	if test -z "$NM_SKIP_CLONE"; then
		rm -rf netmaker-tmp
		mkdir netmaker-tmp
		git clone --single-branch --depth=1 --branch="$BUILD_TAG" https://www.github.com/gravitl/netmaker
	else
		info "Skipping git clone on NM_SKIP_CLONE"
	fi

	pushd netmaker-tmp/netmaker

	if test -z "$NM_SKIP_BUILD"; then
		docker build --no-cache --build-arg version="$IMAGE_TAG" -t "gravitl/netmaker:$IMAGE_TAG" .
	else
		info "Skipping build on NM_SKIP_BUILD"
	fi
	cp compose/docker-compose.yml "$DATA_DIR/docker-compose.yml"
	if [ "$INSTALL_TYPE" = "pro" ]; then
		cp compose/docker-compose.ee.yml "$DATA_DIR/docker-compose.override.yml"
		cp docker/Caddyfile-pro "$DATA_DIR/Caddyfile"
	else
		cp docker/Caddyfile "$DATA_DIR/Caddyfile"
	fi
	cp scripts/.envrc "$DATA_DIR/.envrc"
	cp scripts/netmaker.default.env "$DATA_DIR/netmaker.default.env"
	cp docker/mosquitto.conf "$DATA_DIR/mosquitto.conf"
	cp docker/wait.sh "$DATA_DIR/wait.sh"

	popd
	if test -z "$NM_SKIP_CLONE"; then
		rm -rf netmaker-tmp
	fi
); }

# install_dependencies - install necessary packages to run netmaker
install_dependencies() {
	local command missing_commands=() required_commands=(
		git
		wg
		dig
		jq
		docker
		docker-compose
		grep
		awk
	)
	info "checking dependencies..."

	if [ -f /etc/debian_version ]; then
		dependencies=(git wireguard wireguard-tools dnsutils jq docker.io docker-compose grep gawk)
		update_cmd=(apt update)
		install_cmd=(apt-get install -y)
	elif [ -f /etc/alpine-release ]; then
		dependencies=(git wireguard jq docker.io docker-compose grep gawk)
		update_cmd=(apk update)
		install_cmd=(apk --update add)
	elif [ -f /etc/centos-release ]; then
		dependencies=(git wireguard jq bind-utils docker.io docker-compose grep gawk)
		update_cmd=(yum update)
		install_cmd=(yum install -y)
	elif [ -f /etc/fedora-release ]; then
		dependencies=(git wireguard bind-utils jq docker.io docker-compose grep gawk)
		update_cmd=(dnf update)
		install_cmd=(dnf install -y)
	elif [ -f /etc/redhat-release ]; then
		dependencies=(git wireguard jq docker.io bind-utils docker-compose grep gawk)
		update_cmd=(yum update)
		install_cmd=(yum install -y)
	elif [ -f /etc/arch-release ]; then
		dependencies=(git wireguard-tools dnsutils jq docker.io docker-compose grep gawk)
		update_cmd=(pacman -Sy)
		install_cmd=(pacman -S --noconfirm)
	elif [ "${OS}" = "FreeBSD" ]; then
		dependencies=(git wireguard wget jq docker.io docker-compose grep gawk)
		update_cmd=(pkg update)
		install_cmd=(pkg install -y)
	else
		install_cmd=()
	fi

	for command in "${required_commands[@]}"; do
		if ! command -v "$command" &>/dev/null; then
			missing_commands+=("$command")
		fi
	done
	if test "${#missing_commands[@]}" -gt 0; then
		info "Following binaries are missing:"
		printf "- %s\n" "${missing_commands[@]}" >&2
		if test "${#install_cmd[@]}" == 0; then
			info "OS is not supported for automatic dependencies installation, you need to figure it out yourself"
			exit 1
		fi
	else
		info "All dependencies are available: ${required_commands[*]}"
		return
	fi

	if test -n "$NM_SKIP_DEPS"; then
		info "Skipping dependencies installation due to $NM_SKIP_DEPS"
		return 1
	fi

	set -- "${dependencies[@]}"

	"${update_cmd[@]}"

	while [ -n "$1" ]; do
		if [ "${OS}" = "FreeBSD" ]; then
			is_installed="$(pkg check -d "$1" | grep "Checking" | grep "done")"
			if [ "$is_installed" != "" ]; then
				info "   $1 is installed"
			else
				info "   $1 is not installed. Attempting install."
				"${install_cmd[@]}" "$1"
				sleep 5
				is_installed="$(pkg check -d "$1" | grep "Checking" | grep "done")"
				if [ "$is_installed" != "" ]; then
					info "   $1 is installed"
				elif [ -x "$(command -v "$1")" ]; then
					info "   $1 is installed"
				else
					info "   FAILED TO INSTALL $1"
					info "   This may break functionality."
				fi
			fi
		else
			if [ "${OS}" = "OpenWRT" ] || [ "${OS}" = "TurrisOS" ]; then
				is_installed="$(opkg list-installed "$1" | grep "$1")"
			else
				is_installed="$(dpkg-query -W --showformat='${Status}\n' "$1" | grep "install ok installed")"
			fi
			if [ "${is_installed}" != "" ]; then
				info "     $1 is installed"
			else
				info "     $1 is not installed. Attempting install."
				"${install_cmd[@]}" "$1"
				sleep 5
				if [ "${OS}" = "OpenWRT" ] || [ "${OS}" = "TurrisOS" ]; then
					is_installed="$(opkg list-installed "$1" | grep "$1")"
				else
					is_installed="$(dpkg-query -W --showformat='${Status}\n' "$1" | grep "install ok installed")"
				fi
				if [ "${is_installed}" != "" ]; then
					info "   $1 is installed"
				elif [ -x "$(command -v "$1")" ]; then
					info "   $1 is installed"
				else
					info "   FAILED TO INSTALL $1"
					info "   This may break functionality."
				fi
			fi
		fi
		shift
	done

	info "-----------------------------------------------------"
	info "dependency check complete"
	info "-----------------------------------------------------"
}

make_password() {
	tr -dc A-Za-z0-9 </dev/urandom | head -c "$1"
}

# set_install_vars - sets the variables that will be used throughout installation
set_install_vars() {

	IP_ADDR=$(dig -4 myip.opendns.com @resolver1.opendns.com +short)
	if [ "$IP_ADDR" = "" ]; then
		IP_ADDR="$(curl -s ifconfig.me)"
	fi
	if [ "$NETMAKER_BASE_DOMAIN" = "" ]; then
		NETMAKER_BASE_DOMAIN=nm.$(info "$IP_ADDR" | tr . -).nip.io
	fi
	SERVER_HOST="$IP_ADDR"
	if test -z "$MASTER_KEY"; then
		MASTER_KEY="$(make_password 30)"
	fi
	DOMAIN_TYPE=''
	info "-----------------------------------------------------"
	info "Would you like to use your own domain for netmaker, or an auto-generated domain?"
	info "To use your own domain, add a Wildcard DNS record (e.x: *.netmaker.example.com) pointing to $SERVER_HOST"
	info "IMPORTANT: Due to the high volume of requests, the auto-generated domain has been rate-limited by the certificate provider."
	info "For this reason, we STRONGLY RECOMMEND using your own domain. Using the auto-generated domain may lead to a failed installation due to rate limiting."
	info "-----------------------------------------------------"

	if [ "$AUTO_BUILD" = "on" ]; then
		DOMAIN_TYPE="auto"
	else
		select _ in "Auto Generated ($NETMAKER_BASE_DOMAIN)" "Custom Domain (e.x: netmaker.example.com)"; do
			case $REPLY in
			1)
				info "using $NETMAKER_BASE_DOMAIN for base domain"
				DOMAIN_TYPE="auto"
				break
				;;
			2)
				read -r -p "Enter Custom Domain (make sure  *.domain points to $SERVER_HOST first): " domain
				NETMAKER_BASE_DOMAIN=$domain
				info "using $NETMAKER_BASE_DOMAIN"
				DOMAIN_TYPE="custom"
				break
				;;
			*) info "invalid option $REPLY" ;;
			esac
		done
	fi

	wait_seconds 2

	info "-----------------------------------------------------"
	info "The following subdomains will be used:"
	info "          dashboard.$NETMAKER_BASE_DOMAIN"
	info "                api.$NETMAKER_BASE_DOMAIN"
	info "             broker.$NETMAKER_BASE_DOMAIN"

	if [ "$INSTALL_TYPE" = "pro" ]; then
		info "         prometheus.$NETMAKER_BASE_DOMAIN"
		info "  netmaker-exporter.$NETMAKER_BASE_DOMAIN"
		info "            grafana.$NETMAKER_BASE_DOMAIN"
	fi

	info "-----------------------------------------------------"

	if [[ "$DOMAIN_TYPE" == "custom" ]]; then
		info "before continuing, confirm DNS is configured correctly, with records pointing to $SERVER_HOST"
		confirm
	fi

	wait_seconds 1

	if [ "$INSTALL_TYPE" = "pro" ]; then

		info "-----------------------------------------------------"
		info "Provide Details for pro installation:"
		info "    1. Log into https://app.netmaker.io"
		info "    2. follow instructions to get a license at: https://docs.netmaker.io/ee/ee-setup.html"
		info "    3. Retrieve License and Tenant ID"
		info "    4. note email address"
		info "-----------------------------------------------------"
		LICENSE_KEY=''
		while [ -z "$LICENSE_KEY" ]; do
			read -r -p "License Key: " LICENSE_KEY
		done
		TENANT_ID=''
		while [ -z "${TENANT_ID}" ]; do
			read -r -p "Tenant ID: " TENANT_ID
		done
	fi

	GET_EMAIL=''
	RAND_EMAIL="$(info $RANDOM | md5sum | head -c 16)@email.com"
	# suggest the prev email or a random one
	EMAIL_SUGGESTED="${NM_EMAIL:-$RAND_EMAIL}"
	if [ -z $AUTO_BUILD ]; then
		read -r -p "Email Address for Domain Registration (click 'enter' to use $EMAIL_SUGGESTED): " GET_EMAIL
	fi
	if [ -z "$GET_EMAIL" ]; then
		EMAIL="$EMAIL_SUGGESTED"
		if [ "$EMAIL" = "$NM_EMAIL" ]; then
			info "using config email"
		else
			info "using rand email"
		fi
	else
		EMAIL="$GET_EMAIL"
	fi

	wait_seconds 1

	GET_MQ_USERNAME=''
	GET_MQ_PASSWORD=''
	CONFIRM_MQ_PASSWORD=''

	info "Enter Credentials For MQ..."
	if [ -z $AUTO_BUILD ]; then
		read -r -p "MQ Username (click 'enter' to use 'netmaker'): " GET_MQ_USERNAME
	fi
	if [ -z "$GET_MQ_USERNAME" ]; then
		info "using default username for mq"
		MQ_USERNAME="netmaker"
	else
		# shellcheck disable=SC2034
		MQ_USERNAME="$GET_MQ_USERNAME"
	fi

	if test -z "$MQ_PASSWORD"; then
		MQ_PASSWORD="$(make_password 30)"
	fi

	if [ -z $AUTO_BUILD ]; then
		select _ in "Auto Generated / Config Password" "Input Your Own Password"; do
			case $REPLY in
			1)
				info "using random password for mq"
				break
				;;
			2)
				while true; do
					info "Enter your Password For MQ: "
					read -r -s GET_MQ_PASSWORD
					info "Enter your password again to confirm: "
					read -r -s CONFIRM_MQ_PASSWORD
					if [ "${GET_MQ_PASSWORD}" != "${CONFIRM_MQ_PASSWORD}" ]; then
						info "wrong password entered, try again..."
						continue
					fi
					MQ_PASSWORD="$GET_MQ_PASSWORD"
					info "MQ Password Saved Successfully!!"
					break
				done
				break
				;;
			*) info "invalid option $REPLY" ;;
			esac
		done
	fi

	wait_seconds 2

	info "-----------------------------------------------------------------"
	info "                SETUP ARGUMENTS"
	info "-----------------------------------------------------------------"
	info "        domain: $NETMAKER_BASE_DOMAIN"
	info "         email: $EMAIL"
	info "     public ip: $SERVER_HOST"
	if [ "$INSTALL_TYPE" = "pro" ]; then
		info "       license: $LICENSE_KEY"
		info "    account id: $TENANT_ID"
	fi
	info "-----------------------------------------------------------------"
	info "Confirm Settings for Installation"
	info "- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -"

	if [ ! "$BUILD_TYPE" = "local" ]; then
		IMAGE_TAG="$LATEST"
	fi

	confirm
}

# install_netmaker - sets the config files and starts docker-compose
install_netmaker() {

	info "-----------------------------------------------------------------"
	info "Beginning installation..."
	info "-----------------------------------------------------------------"

	wait_seconds 3

	info "Pulling config files..."

	if [ "$BUILD_TYPE" = "local" ]; then
		local_install_setup
	else
		local BASE_URL="https://raw.githubusercontent.com/gravitl/netmaker/$BUILD_TAG"

		local COMPOSE_URL="$BASE_URL/compose/docker-compose.yml"
		local CADDY_URL="$BASE_URL/docker/Caddyfile"
		if [ "$INSTALL_TYPE" = "pro" ]; then
			local COMPOSE_OVERRIDE_URL="$BASE_URL/compose/docker-compose.pro.yml"
			local CADDY_URL="$BASE_URL/docker/Caddyfile-pro"
		fi
		wget -qO "$DATA_DIR"/docker-compose.yml "$COMPOSE_URL"
		if test -n "$COMPOSE_OVERRIDE_URL"; then
			wget -qO "$DATA_DIR"/docker-compose.override.yml "$COMPOSE_OVERRIDE_URL"
		fi
		wget -qO "$DATA_DIR"/Caddyfile "$CADDY_URL"
		wget -qO "$DATA_DIR"/netmaker.default.env "$BASE_URL/scripts/netmaker.default.env"
		wget -qO "$DATA_DIR"/mosquitto.conf "$BASE_URL/docker/mosquitto.conf"
		wget -qO "$DATA_DIR"/wait.sh "$BASE_URL/docker/wait.sh"
	fi

	chmod +x "$DATA_DIR"/wait.sh
	mkdir -p /etc/netmaker

	save_config
	# link .env to the user config
	ln -fs "$CONFIG_FILENAME" "$DATA_DIR/.env"

	info "Starting containers..."

	# increase the timeouts
	export DOCKER_CLIENT_TIMEOUT=120
	export COMPOSE_HTTP_TIMEOUT=120

	# start docker and rebuild containers / networks
	docker-compose up -d --force-recreate
	wait_seconds 2

}

# test_connection - tests to make sure Caddy has proper SSL certs
test_connection() {

	info "Testing Caddy setup (please be patient, this may take 1-2 minutes)"
	for i in 1 2 3 4 5 6 7 8; do
		curlresponse="$(curl -vIs "https://api.${NETMAKER_BASE_DOMAIN}" 2>&1)"

		if [[ "$i" == 8 ]]; then
			info "    Caddy is having an issue setting up certificates, please investigate (docker logs caddy)"
			info "    Exiting..."
			exit 1
		elif [[ "$curlresponse" == *"failed to verify the legitimacy of the server"* ]]; then
			info "    Certificates not yet configured, retrying..."

		elif [[ "$curlresponse" == *"left intact"* ]]; then
			info "    Certificates ok"
			break
		else
			secs="$(("$i" * 5 + 10))"
			info "    Issue establishing connection...retrying in $secs seconds..."
		fi
		sleep "$secs"
	done

}

# setup_mesh - sets up a default mesh network on the server
setup_mesh() {
	local networkCount

	wait_seconds 5

	networkCount="$(nmctl network list -o json | jq '. | length')"

	# add a network if none present
	if [ "$networkCount" -lt 1 ]; then
		info "Creating netmaker network (10.101.0.0/16)"

		# TODO causes "Error Status: 400 Response: {"Code":400,"Message":"could not find any records"}"
		nmctl network create --name netmaker --ipv4_addr 10.101.0.0/16

		wait_seconds 5
	fi
}

get_enrollment_key() {
	info "Obtaining a netmaker enrollment key..."

	if test -z "$TOKEN"; then
		TOKEN="$(nmctl enrollment_key create --tags netmaker --unlimited --networks netmaker | jq -r '.token')"
	fi
	if test -z "$TOKEN"; then
		info "Error creating an enrollment key"
		exit 1
	else
		info "Enrollment key ready"
	fi
	info -n "$TOKEN"
	wait_seconds 3
}

# print_success - prints a success message upon completion
print_success() {
	info "-----------------------------------------------------------------"
	info "-----------------------------------------------------------------"
	info "Netmaker setup is now complete. You are ready to begin using Netmaker."
	info "Visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
	info "-----------------------------------------------------------------"
	info "-----------------------------------------------------------------"
}

cleanup() {
	# remove the existing netclient's instance from the existing network
	if test -z "$NM_SKIP_CLIENT"; then
		local node_id
		node_id="$(netclient list | jq -r '.[0].node_id' 2>/dev/null || :)"
		if test -n "$node_id"; then
			info "De-registering the existing netclient..."
			nmctl node delete netmaker "$node_id" >/dev/null 2>&1
		fi
	fi

	info "Stopping all containers..."
	local containers=("mq" "netmaker-ui" "coredns" "turn" "caddy" "netmaker" "netmaker-exporter" "prometheus" "grafana")
	local running exists
	for name in "${containers[@]}"; do
		running=$(docker ps "$name" || :)
		exists=$(docker ps -a "$name" || :)
		if test -n "$running"; then
			docker stop "$name"
		fi
		if test -n "$exists"; then
			docker rm "$name"
		fi
	done
}

main() {
	# 1. print netmaker logo
	print_logo

	# 2. prepare configuration & setup the build instruction
	configure "$@"

	# 3. install necessary packages
	install_dependencies || :

	# 4. get user input for variables
	set_install_vars

	cleanup || :

	# 5. get and set config files, startup docker-compose
	install_netmaker

	# 6. make sure Caddy certs are working
	test_connection

	# 7. create a default mesh network for netmaker
	setup_mesh

	# 8. add netclient to docker-compose and start it up
	setup_netclient

	# 9. make the netclient a default host and ingress gw
	configure_netclient

	# 10. print success message
	print_success
}

main "${ARGV[@]}"
