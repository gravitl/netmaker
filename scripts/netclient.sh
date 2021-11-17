#!/bin/sh
echo "[netclient] joining network"

if [ -z "${SLEEP}" ]; then
    SLEEP=10
fi

TOKEN_CMD=""
if [ "$TOKEN" != "" ]; then
    TOKEN_CMD="-t $TOKEN"
fi

/root/netclient join $TOKEN_CMD -daemon off -dnson no

echo "[netclient] Starting netclient checkin"
# loop and call checkin -n all
while [ 1 ]; do
    # add logs to netclient.logs
    /root/netclient checkin -n all
    sleep $SLEEP
done
echo "[netclient] exiting"
