#!/bin/bash

#Define cleanup
cleanup() {
    nets=($(wg show interfaces))
    for net in ${nets[@]}; do
        echo "deleting interface" $net
        ip link del $net
    done
}

#Trap SigTerm
trap 'cleanup' SIGTERM

echo "[netclient] joining network"

if [ -z "${SLEEP}" ]; then
    SLEEP=10
fi

TOKEN_CMD=""
if [ "$TOKEN" != "" ]; then
    TOKEN_CMD="-t $TOKEN"
fi

/root/netclient join $TOKEN_CMD -udpholepunch no
if [ $? -ne 0 ]; then { echo "Failed to join, quitting." ; exit 1; } fi

echo "[netclient] Starting netclient daemon"

/root/netclient daemon &

wait $!
echo "[netclient] exiting"
