#!/bin/sh

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root"
   exit 1
fi

echo "checking dependencies..."

OS=$(uname)

if [ -f /etc/debian_version ]; then
	install_cmd='apt-get install -y'
elif [ -f /etc/alpine-release ]; then
	install_cmd='apk --update add'
elif [ -f /etc/centos-release ]; then
	install_cmd='yum install -y'
elif [ -f /etc/fedora-release ]; then
	install_cmd='dnf install -y'
elif [ "${OS}" = "FreeBSD" ]; then
	install_cmd='pkg install -y'
else
	install_cmd=''
fi

if [ -z "${install_cmd}" ]; then
        echo "OS unsupported for automatic dependency install"
	exit 1
fi
dependencies="wireguard"
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
		is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
		if [ "${is_installed}" = "install ok installed" ]; then
			echo "    " $1 is installed
		else
			echo "    " $1 is not installed. Attempting install.
			${install_cmd} $1
			sleep 5
			is_installed=$(dpkg-query -W --showformat='${Status}\n' $1 | grep "install ok installed")
				if [ "${is_installed}" = "install ok installed" ]; then
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

case $(uname | tr '[:upper:]' '[:lower:]') in
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
                        x86_32)
                                dist=netclient-32
                        ;;
 			arm64)
				dist=netclient-arm64
			;;
			aarch64)
                                dist=netclient-arm64
			;;
			armv7l)
                                dist=netclient-armv7
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
                        x86_32)
                                dist=netclient-freebsd-32
                        ;;
 			arm64)
				dist=netclient-freebsd-arm64
			;;
			aarch64)
                                dist=netclient-freebsd-arm64
			;;
			armv7l)
                                dist=netclient-freebsd-armv7
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
if curl --output /dev/null --silent --head --fail "$url"; then
	echo "Downloading $dist $VERSION"
	wget -nv -O netclient $url
else
	echo "Downloading $dist latest"
	wget -nv -O netclient https://github.com/gravitl/netmaker/releases/download/latest/$dist
fi

chmod +x netclient

EXTRA_ARGS=""
if [ "${OS}" = "FreeBSD" ]; then
	EXTRA_ARGS="--daemon=off"
fi

if [ -z "${NAME}" ]; then
  ./netclient join -t $KEY $EXTRA_ARGS
else
  ./netclient join -t $KEY --name $NAME $EXTRA_ARGS
fi

if [ "${OS}" = "FreeBSD" ]; then
	mv ./netclient /etc/netclient/netclient
	cat << 'END_OF_FILE' > ./netclient.service.tmp
#!/bin/sh

# PROVIDE: netclient
# REQUIRE: LOGIN DAEMON NETWORKING SERVERS FILESYSTEM
# BEFORE:  
# KEYWORD: shutdown
. /etc/rc.subr

name="netclient"
rcvar=netclient_enable
pidfile="/var/run/${name}.pid"
command="/usr/sbin/daemon"
command_args="-c -f -P ${pidfile} -R 10 -t "Netclient" -u root -o /etc/netclient/netclient.log /etc/netclient/netclient checkin -n all"

load_rc_config $name
run_rc_command "$1"

END_OF_FILE
	sudo mv ./netclient.service.tmp /usr/local/etc/rc.d/netclient
	sudo chmod +x /usr/local/etc/rc.d/netclient
	sudo /usr/local/etc/rc.d/netclient enable
	sudo /usr/local/etc/rc.d/netclient start
else
	rm -f netclient
fi
