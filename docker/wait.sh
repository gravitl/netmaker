#!/bin/sh

wait_for_netmaker() {
  until curl --output /dev/null --silent --fail --head \
    --location "${NETMAKER_SERVER_HOST}/api/server/health"; do
    echo "Waiting for netmaker server to startup"
    sleep 1
  done
}

main() {
  # wait for netmaker to startup
  apk add curl
  wait_for_netmaker
}

main "${@}"