#!/bin/bash

PUBKEY="DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
IPADDR="69.173.21.202"
MACADDRESS="59:2a:9c:d4:e2:49"
ACCESSKEY="tMwl7zMLLnqP7sE5"
PASSWORD="ppppppp"

generate_post_json ()
{
  cat <<EOF
{
  "endpoint": "$IPADDR",
  "publickey": "$PUBKEY",
  "macaddress": "$MACADDRESS",
  "password": "$PASSWORD",
  "localaddress": "172.16.16.1",
  "accesskey": "$ACCESSKEY"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer mastertoken" localhost:8081/api/skynet/nodes

