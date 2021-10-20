# If running userspace wireguard in Docker, create missing tun device.
if [ ! -d /dev/net ]; then mkdir /dev/net; fi
if [ ! -e /dev/net/tun ]; then  mknod /dev/net/tun c 10 200; fi

# Wait and then run netmaker.
/bin/sh -c "sleep 3; ./netmaker"