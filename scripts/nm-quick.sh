#!/bin/bash
echo "checking for root permissions..."

if [ $EUID -ne 0 ]; then
   echo "This script must be run as root" 
   exit 1
fi


echo "checking dependencies..."

declare -A osInfo;
osInfo[/etc/debian_version]="apt-get install -y"
osInfo[/etc/alpine-release]="apk --update add"
osInfo[/etc/centos-release]="yum install -y"
osInfo[/etc/fedora-release]="dnf install -y"

for f in ${!osInfo[@]}
do
    if [[ -f $f ]];then
        install_cmd=${osInfo[$f]}
    fi
done

dependencies=("docker.io" "docker-compose" "wireguard")

for dependency in ${dependencies[@]}; do
    is_installed=$(dpkg-query -W --showformat='${Status}\n' ${dependency} | grep "install ok installed")

    if [ "${is_installed}" == "install ok installed" ]; then
        echo "    " ${dependency} is installed
    else
            echo "    " ${dependency} is not installed. Attempting install.
            ${install_cmd} ${dependency}
            sleep 5
            is_installed=$(dpkg-query -W --showformat='${Status}\n' ${dependency} | grep "install ok installed")
            if [ "${is_installed}" == "install ok installed" ]; then
                echo "    " ${dependency} is installed
            elif [ -x "$(command -v ${dependency})" ]; then
                echo "    " ${dependency} is installed
            else
                echo "    " failed to install ${dependency}. Exiting.
                exit 1
            fi
    fi
done




set -e

echo "setting public ip values..."

NETMAKER_BASE_DOMAIN=nm.$(curl -s ifconfig.me | tr . -).nip.io
COREDNS_IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p')
SERVER_PUBLIC_IP=$(curl -s ifconfig.me)
REPLACE_MASTER_KEY=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo '')
EMAIL="fake@email.com"

if [ $# -eq 2 ]
then
arg1=$( echo $1 | awk -F"=" '{print $2}')
arg2=$( echo $2 | awk -F"=" '{print $2}')
value=$( echo $1 | awk -F"=" '{print $1}')

if [ $value == domain ];then
	echo "Paramater NETMAKER_BASE_DOMAIN is $arg1"
	NETMAKER_BASE_DOMAIN=$arg1
	echo "Paramater EMAIL is $arg2"
	EMAIL=$arg2
	else
	echo "Paramater NETMAKER_BASE_DOMAIN is $arg2"
	NETMAKER_BASE_DOMAIN=$arg2
	echo "Paramater EMAIL is $arg1"
	EMAIL=$arg1
fi

elif [ $# -eq 1 ];then
	check=$( echo $1 | awk -F"=" '{print $1}')
	value=$( echo $1 | awk -F"=" '{print $2}')
if [ $check == domain ];then
	echo "Paramater NETMAKER_BASE_DOMAIN is $value"
	NETMAKER_BASE_DOMAIN=$value
else
	echo "Paramater EMAIL is $value"
	EMAIL=$value
fi
fi

echo "        domain: $NETMAKER_BASE_DOMAIN"
echo "    coredns ip: $COREDNS_IP"
echo "     public ip: $SERVER_PUBLIC_IP"
echo "    master key: $REPLACE_MASTER_KEY"


echo "setting caddyfile..."


wget -O /root/Caddyfile https://raw.githubusercontent.com/gravitl/netmaker/master/docker/Caddyfile
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/Caddyfile
sed -i "s/YOUR_EMAIL/$EMAIL/g" /root/Caddyfile


echo "setting docker-compose..."

wget -O /root/docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.caddy.yml
sed -i "s/NETMAKER_BASE_DOMAIN/$NETMAKER_BASE_DOMAIN/g" /root/docker-compose.yml
sed -i "s/SERVER_PUBLIC_IP/$SERVER_PUBLIC_IP/g" /root/docker-compose.yml
sed -i "s/COREDNS_IP/$COREDNS_IP/g" /root/docker-compose.yml
sed -i "s/REPLACE_MASTER_KEY/$REPLACE_MASTER_KEY/g" /root/docker-compose.yml

echo "starting containers..."

docker-compose -f /root/docker-compose.yml up -d

cat << "EOF"


    ______     ______     ______     __   __   __     ______   __                        
   /\  ___\   /\  == \   /\  __ \   /\ \ / /  /\ \   /\__  _\ /\ \                       
   \ \ \__ \  \ \  __<   \ \  __ \  \ \ \'/   \ \ \  \/_/\ \/ \ \ \____                  
    \ \_____\  \ \_\ \_\  \ \_\ \_\  \ \__|    \ \_\    \ \_\  \ \_____\                 
     \/_____/   \/_/ /_/   \/_/\/_/   \/_/      \/_/     \/_/   \/_____/                 
                                                                                         
 __   __     ______     ______   __    __     ______     __  __     ______     ______    
/\ "-.\ \   /\  ___\   /\__  _\ /\ "-./  \   /\  __ \   /\ \/ /    /\  ___\   /\  == \   
\ \ \-.  \  \ \  __\   \/_/\ \/ \ \ \-./\ \  \ \  __ \  \ \  _"-.  \ \  __\   \ \  __<   
 \ \_\\"\_\  \ \_____\    \ \_\  \ \_\ \ \_\  \ \_\ \_\  \ \_\ \_\  \ \_____\  \ \_\ \_\ 
  \/_/ \/_/   \/_____/     \/_/   \/_/  \/_/   \/_/\/_/   \/_/\/_/   \/_____/   \/_/ /_/ 
                                                                                         															 

EOF

echo "             finished installing"
echo " "
echo "             visit dashboard.$NETMAKER_BASE_DOMAIN to log in"
echo " "
echo " "
