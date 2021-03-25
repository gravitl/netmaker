#!/bin/bash

PUBKEY="DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
IPADDR="67.169.21.168"
MACADDRESS="56:2a:9c:d4:e2:15"
ACCESSKEY="secretkey"
PASSWORD="password"

generate_post_json ()
{
  cat <<EOF
{
  "endpoint": "$IPADDR",
  "publickey": "$PUBKEY",
  "macaddress": "$MACADDRESS",
  "password": "$PASSWORD"
}
EOF
}

POST_JSON=$(generate_post_json)

echo $POST_JSON

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer mastertoken" localhost:8081/api/doofusnet/nodes
