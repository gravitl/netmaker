#!/bin/sh

mkdir -p /root/gravitl/netclient_0.VERSION_amd64/sbin
mkdir -p /root/gravitl/netclient_0.VERSION_amd64/var/lib/systemd/system
mkdir -p /root/gravitl/netclient_0.VERSION_amd64/DEBIAN
mkdir -p /root/gravitl/netclient_0.VERSION_arm64/sbin
mkdir -p /root/gravitl/netclient_0.VERSION_arm64/var/lib/systemd/system
mkdir -p /root/gravitl/netclient_0.VERSION_arm64/DEBIAN

cat << EOF > /root/gravitl/netclient_0.VERSION_amd64/DEBIAN/control
Package: netclient
Version: VERSION
Maintainer: info@gravitl.com
Depends: wireguard-tools
Architecture: amd64
Homepage: https://github.com/gravitl/netmaker
Description: netclient daemon - a platform for modern, blazing fast virtual networks
EOF

cat << EOF > /root/gravitl/netclient_0.VERSION_arm64/DEBIAN/control
Package: netclient
Version: VERSION
Maintainer: info@gravitl.com
Depends: wireguard-tools
Architecture: arm64
Homepage: https://github.com/gravitl/netmaker
Description: netclient daemon - a platform for modern, blazing fast virtual networks
EOF

wget https://github.com/gravitl/netmaker/releases/download/vVERSION/netclient -O /root/gravitl/netclient_0.VERSION_amd64/sbin/netclient
wget https://github.com/gravitl/netmaker/releases/download/vVERSION/netclient-arm64 -O /root/gravitl/netclient_0.VERSION_arm64/sbin/netclient
wget https://raw.githubusercontent.com/gravitl/netmaker/master/netclient/build/netclient.service -O /root/gravitl/netclient_0.VERSION_amd64/var/lib/systemd/system/netclient.service
cp /root/gravitl/netclient_0.VERSION_amd64/var/lib/systemd/system/netclient.service /root/gravitl/netclient_0.VERSION_arm64/var/lib/systemd/system/netclient.service

dpkg --build /root/gravitl/netclient_0.VERSION_amd64
dpkg --build /root/gravitl/netclient_0.VERSION_arm64

mkdir -p /var/apt-repo/pool/main
mkdir -p /var/apt-repo/dists/stable/main/binary-amd64
mkdir -p /var/apt-repo/dists/stable/main/binary-arm64

cp /root/gravitl/netclient_0.VERSION_amd64.deb /var/apt-repo/pool/main
cp /root/gravitl/netclient_0.VERSION_arm64.deb /var/apt-repo/pool/main

cd /var/apt-repo
dpkg-scanpackages --arch amd64 -m pool/ > dists/stable/main/binary-amd64/Packages
dpkg-scanpackages --arch arm64 -m pool/ > dists/stable/main/binary-arm64/Packages
cat dists/stable/main/binary-amd64/Packages | gzip -9 > dists/stable/main/binary-amd64/Packages.gz
cat dists/stable/main/binary-arm64/Packages | gzip -9 > dists/stable/main/binary-arm64/Packages.gz

cd dists/stable
/root/generate_release.sh > Release

cat /var/apt-repo/dists/stable/Release | gpg --default-key gravitl -abs > /var/apt-repo/dists/stable/Release.gpg
cat /var/apt-repo/dists/stable/Release | gpg --default-key gravitl -abs --clearsign > /var/apt-repo/dists/stable/InRelease

if test -f /var/apt-repo/gpg.key  ; then
    rm /var/apt-repo/gpg.key
fi
gpg --export -a --output /var/apt-repo/gpg.key gravitl
cat <<EOF > /var/apt-repo/debian.deb.txt
# Source: netclient
# Site: https://github.com/gravitl/netmaker
# Repository: Netmaker / stable
# Description:  a platform for modern, blazing fast virtual networks

deb [arch=amd64] https:apt.clustercat.com stable main
EOF


