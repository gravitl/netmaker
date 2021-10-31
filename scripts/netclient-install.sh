#!/bin/sh

if [ $(id -u) -ne 0 ]; then
   echo "This script must be run as root" 
   exit 1
fi

echo "checking dependencies..."

if [ -f /etc/debian_version ]; then
	install_cmd='apt-get install -y'
elif [ -f /etc/alpine-release ]; then
	install_cmd='apk --update add'
elif [ -f /etc/centos-release ]; then
	install_cmd='yum install -y'
elif [ -f /etc/fedora-release ]; then
	install_cmd='dnf install -y'
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
	shift
done

set -e

[ -z "$KEY" ] && KEY=nokey;
[ -z "$VERSION" ] && echo "no \$VERSION provided, fallback to latest" && VERSION=latest;

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
			arm*)
				dist=netclient-$CPU_ARCH
            		;;
			*)
				fatal "$CPU_ARCH : cpu architecture not supported"
    		esac
	;;
	darwin)
        	dist=netclient-darwin
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
sudo ./netclient join -t $KEY
rm -f netclient
