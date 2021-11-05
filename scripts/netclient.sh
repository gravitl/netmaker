#!/bin/sh
echo "[netclient] joining network"

/etc/netclient/netclient join -t $NETCLIENT_ACCESSTOKEN -daemon off -dnson no

echo "[netclient] Starting netclient checkin"
# loop and call checkin -n all
while [ 1 ]; do
    # add logs to netclient.logs
    /etc/netclient/netclient checkin -n all
    sleep $SLEEP
done
echo "[netclient] exiting"
