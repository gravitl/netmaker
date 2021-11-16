#!/bin/bash

PUBKEY="DM5qhLAE20EG9BbfBEger+Ac9D2NDOwCtY1rbYDLf34="
IPADDR="70.173.21.212"
MACADDRESS="59:23:9c:f2:e4:49"
ACCESSKEY="DYrXDkNuXC3XQ27J"
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

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/nodes/skynet

