#!/bin/ash

wait_for_netmaker() {
  echo "SERVER: ${NETMAKER_SERVER_HOST}"
  until curl --output /dev/null --silent --fail --head \
    --location "${NETMAKER_SERVER_HOST}/api/server/health"; do
    echo "Waiting for netmaker server to startup"
    sleep 1
  done
}

main(){
 # wait for netmaker to startup
 apk add curl
 wait_for_netmaker
 echo "Starting MQ..."
 # Run the main container command.
 /docker-entrypoint.sh
 /usr/sbin/mosquitto -c /mosquitto/config/mosquitto.conf

}

main "${@}"
