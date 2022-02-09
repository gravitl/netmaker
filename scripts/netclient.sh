#!/bin/sh

echo "[netclient] joining network"

if [ -z "${SLEEP}" ]; then
    SLEEP=10
fi

TOKEN_CMD=""
if [ "$TOKEN" != "" ]; then
    TOKEN_CMD="-t $TOKEN"
fi

/root/netclient join $TOKEN_CMD -daemon off -dnson no -udpholepunch no
if [ $? -ne 0 ]; then { echo "Failed to join, quitting." ; exit 1; } fi

echo "[netclient] Starting netclient daemon"

/root/netclient daemon

echo "[netclient] exiting"
