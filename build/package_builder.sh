#!/bin/bash

# set environment var VERSION = netclient version without leading v  --- 0.13.0 vice v0.13.0

# set version
sed -i "s/VERSION/$VERSION/g" ./build/make_deb.sh
sed -i "s/VERSION/$VERSION/g" ./build/generate_release.sh
sed -i "s/VERSION/$VERSION/g" ./build/make_rpm.sh
sed -i "s/VERSION/$VERSION/g" ./build/netclient.spec

# build and deploy deb repo
scp build/make_deb.sh fileserver.clustercat.com:~/
scp build/generate_release.sh fileserver.clustercat.com:~/
ssh -t fileserver.clustercat.com /root/make_deb.sh

# build and deploy rpm repo
scp build/make_rpm.sh fileserver.clustercat.com:~/
scp build/netclient.spec fileserver.clustercat.com:~/
scp build/.rpmmacros fileserver.clustercat.com:~/
ssh -t fileserver.clustercat.com /root/make_rpm.sh

