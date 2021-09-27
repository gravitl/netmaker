# create a logs file
sudo touch /etc/netclient/netclient.logs
echo "[netclient] created logs file in /etc/netclient/netclient.logs"
echo "[netclient] Starting netclient checkins"
# loop and call checkin -n all
while [ 1 ]; do
    # add logs to netclient.logs
    sudo /etc/netclient/netclient checkin -n all >> /etc/netclient/netclient.logs 2&1>
    sleep 15
done &
echo "[netclient] exiting"
