while [ 1 ]; do
    /etc/netclient/netclient checkin -n all >> /etc/netclient/netclient.logs 2&1>
    sleep 15
done &
