#!/bin/bash

PUBKEY="DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34="
IPADDR="69.169.21.167"
MACADDRESS="56:2a:9c:d4:e2:16"
ACCESSKEY="HzIeRewZEyMWtUSF"
PASSWORD="password"
LOCALADDRESS="192.168.1.21"

generate_post_json ()
{
  cat <<EOF
{
  "endpoint": "$IPADDR",
  "publickey": "$PUBKEY",
  "macaddress": "$MACADDRESS",
  "password": "$PASSWORD",
  "localaddress": "$LOCALADDRESS",
  "accesskey": "$ACCESSKEY"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer mastertoken" localhost:8082/api/nodes/skynet

PUBKEY="ap200E90in4uyIyR0b3wuX24ZAp0WFL8q37UtL3CWFI="
IPADDR="70.169.21.168"
MACADDRESS="82:1a:33:d7:e1:96"
PASSWORD="password"
LOCALADDRESS="192.73.1.2"

POST_JSON=$(generate_post_json)


curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer mastertoken" localhost:8082/api/nodes/skynet

PUBKEY="CAHIgkHXOsTEmY9XKhEI3CO5iYAo0X4U/yTX+L/yJ2E="
IPADDR="71.169.21.169"
MACADDRESS="e8:d0:fc:fe:1f:01"
PASSWORD="password"
LOCALADDRESS="192.73.1.3"

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer mastertoken" localhost:8082/api/nodes/skynet
