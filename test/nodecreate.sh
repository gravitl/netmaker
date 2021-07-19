#!/bin/bash

PUBKEY="DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
IPADDR="69.173.21.202"
MACADDRESS="59:2a:9c:d4:e2:49"
ACCESSKEY="9ktiHcUWay2MSKsY"
PASSWORD="ppppppp"

generate_post_json ()
{
  cat <<EOF
{
  "endpoint": "$IPADDR",
  "publickey": "$PUBKEY",
  "macaddress": "$MACADDRESS",
  "password": "$PASSWORD",
  "localaddress": "172.123.123.3",
  "accesskey": "$ACCESSKEY"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/nodes/default

