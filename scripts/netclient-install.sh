#!/bin/sh

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

echo "checking dependencies..."

OS=$(uname)

if [ -f /etc/debian_version ]; then
	dependencies="wireguard wireguard-tools"
	update_cmd='apt update'
	install_cmd='apt-get install -y'
elif [ -f /etc/alpine-release ]; then
	dependencies="wireguard"
	update_cmd='apk update'
	install_cmd='apk --update add'
elif [ -f /etc/centos-release ]; then
	dependencies="wireguard"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/fedora-release ]; then
	dependencies="wireguard"
	update_cmd='dnf update'
	install_cmd='dnf install -y'
elif [ -f /etc/redhat-release ]; then
	dependencies="wireguard"
	update_cmd='yum update'
	install_cmd='yum install -y'
elif [ -f /etc/arch-release ]; then
    	dependecies="wireguard-tools"
	update_cmd='pacman -Sy'
	install_cmd='pacman -S --noconfirm'
elif [ "${OS}" = "FreeBSD" ]; then
	dependencies="wireguard wget"
	update_cmd='pkg update'
	install_cmd='pkg install -y'
elif [ -f /etc/openwrt_release ]; then
	dependencies="wireguard-tools bash"
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

${update_cmd}

set -- $dependencies
while [ -n "$1" ]; do
    echo $1
	if [ "${OS}" = "FreeBSD" ]; then
		is_installed=$(pkg check -d $1 | grep "Checking" | grep "done")
		if [ "$is_installed" != "" ]; then
			echo "    " $1 is installed
		else
			echo "    " $1 is not installed. Attempting install.
			${install_cmd} $1
			sleep 5
			is_installed=$(pkg check -d $1 | grep "Checking" | grep "done")
			if [ "$is_installed" != "" ]; then
				echo "    " $1 is installed
			elif [ -x "$(command -v $1)" ]; then
				echo "    " $1 is installed
			else
				echo "    " FAILED TO INSTALL $1
				echo "    " This may break functionality.
			fi
		fi	
	else
		if [ "${OS}" = "OpenWRT" ]; then
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
			if [ "${OS}" = "OpenWRT" ]; then
				is_installed=$(opkg list-installed $1 | grep $1)
			else
				is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
			fi
			if [ "${is_installed}" != "" ]; then
				echo "    " $1 is installed
			elif [ -x "$(command -v $1)" ]; then
				echo "    " $1 is installed
			else
				echo "    " FAILED TO INSTALL $1
				echo "    " This may break functionality.
			fi
		fi
	fi
	shift
done

set -e

[ -z "$KEY" ] && KEY=nokey;
[ -z "$VERSION" ] && echo "no \$VERSION provided, fallback to latest" && VERSION=latest;
[ "latest" != "$VERSION" ] && [ "v" != `echo $VERSION | cut -c1` ] && VERSION="v$VERSION"
[ -z "$NAME" ] && NAME="";

dist=netclient

echo "OS Version = $(uname)"
echo "Netclient Version = $VERSION"

case $(uname | tr A-Z a-z) in
	linux*)
		if [ -z "$CPU_ARCH" ]; then
			CPU_ARCH=$(uname -m)
		fi
		case $CPU_ARCH in
			amd64)
				dist=netclient
			;;
			x86_64)
				dist=netclient
			;;
 			arm64)
				dist=netclient-arm64
			;;
			aarch64)
                dist=netclient-arm64
			;;
			armv6l)
                dist=netclient-arm6
			;;
			armv7l)
                dist=netclient-arm7
			;;
			arm*)
				dist=netclient-$CPU_ARCH
			;;
            mipsle)
                dist=netclient-mipsle
			;;
			*)
				fatal "$CPU_ARCH : cpu architecture not supported"
    		esac
	;;
	darwin)
        dist=netclient-darwin
	;;
	Darwin)
        dist=netclient-darwin
	;;
	freebsd*)
		if [ -z "$CPU_ARCH" ]; then
			CPU_ARCH=$(uname -m)
		fi
		case $CPU_ARCH in
			amd64)
				dist=netclient-freebsd
			;;
			x86_64)
				dist=netclient-freebsd
			;;
 			arm64)
				dist=netclient-freebsd-arm64
			;;
			aarch64)
                dist=netclient-freebsd-arm64
			;;
			armv7l)
                dist=netclient-freebsd-arm7
			;;
			arm*)
				dist=netclient-freebsd-$CPU_ARCH
            ;;
			*)
				fatal "$CPU_ARCH : cpu architecture not supported"
    		esac
	;;
esac

echo "Binary = $dist"

url="https://github.com/gravitl/netmaker/releases/download/$VERSION/$dist"
curl_opts='-nv'
if [ "${OS}" = "OpenWRT" ]; then
	curl_opts='-q'
fi

if curl --output /dev/null --silent --head --fail "$url"; then
	echo "Downloading $dist $VERSION"
	wget $curl_opts -O netclient $url
else
	echo "Downloading $dist latest"
	wget $curl_opts -O netclient https://github.com/gravitl/netmaker/releases/latest/download/$dist
fi

chmod +x netclient

EXTRA_ARGS=""
if [  "${OS}" = "OpenWRT" ]; then
	EXTRA_ARGS="--daemon=off"
fi

if [ "${KEY}" != "nokey" ]; then
  if [ -z "${NAME}" ]; then
    ./netclient join -t $KEY $EXTRA_ARGS
  else
    ./netclient join -t $KEY --name $NAME $EXTRA_ARGS
  fi
fi

if [ "${OS}" = "FreeBSD" ]; then
  if ! [ -x /usr/sbin/netclient ]; then
    echo "Moving netclient executable to \"/usr/sbin/netclient\""
    mv netclient /usr/sbin  
  else
    echo "Netclient already present."
  fi
fi

if [ "${OS}" = "OpenWRT" ]; then
	mv ./netclient /sbin/netclient
	cat << 'END_OF_FILE' > ./netclient.service.tmp
#!/bin/sh /etc/rc.common

EXTRA_COMMANDS="status"
EXTRA_HELP="        status      Check service is running"
START=99

LOG_FILE="/tmp/netclient.logs"

start() {
  if [ ! -f "${LOG_FILE}" ];then
      touch "${LOG_FILE}"
  fi
  local PID=$(ps|grep "netclient daemon"|grep -v grep|awk '{print $1}')
  if [ "${PID}" ];then
    echo "service is running"
    return
  fi
  bash -c "do /sbin/netclient daemon  >> ${LOG_FILE} 2>&1;\
           if [ $(ls -l ${LOG_FILE}|awk '{print $5}') -gt 10240000 ];then tar zcf "${LOG_FILE}.tar" -C / "tmp/netclient.logs"  && > $LOG_FILE;fi;done &"
  echo "start"
}

stop() {
  pids=$(ps|grep "netclient daemon"|grep -v grep|awk '{print $1}')
  for i in "${pids[@]}"
  do
	if [ "${i}" ];then
		kill "${i}"
	fi
  done
  echo "stop"
}

status() {
  local PID=$(ps|grep "netclient daemon"|grep -v grep|awk '{print $1}')
  if [ "${PID}" ];then
    echo -e "netclient[${PID}] is running \n"
  else
    echo -e "netclient is not running \n"
  fi
}

END_OF_FILE
	mv ./netclient.service.tmp /etc/init.d/netclient
	chmod +x /etc/init.d/netclient
	/etc/init.d/netclient enable
	/etc/init.d/netclient start
else 
	rm -f netclient
fi

