echo "[netclient] joining network"

/root/netclient join -t $ACCESS_TOKEN -daemon off -dnson no

cp netclient /etc/netclient/netclient

echo "[netclient] Starting netclient checkin"
# loop and call checkin -n all
while [ 1 ]; do
    # add logs to netclient.logs
    /etc/netclient/netclient checkin -n $NETWORK
    sleep $SLEEP
done
echo "[netclient] exiting"

