rm -rf /etc/systemd/system/netclient-default.timer
rm -rf /etc/systemd/system/netclient@.service 
rm -rf /etc/netclient/
systemctl daemon-reload
ip link del nm-default
ip link del nm-grpc-wg
docker-compose -f /root/netmaker/compose/docker-compose.yml down --volumes
