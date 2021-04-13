#!/bin/bash

NAME="doofusnet"
ADDRESSRANGE="10.69.0.0/16"

generate_post_json ()
{
  cat <<EOF
{
  "netid": "$NAME",
  "addressrange": "$ADDRESSRANGE"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/networks

NAME="skynet"
ADDRESSRANGE="100.70.0.0/14"

POST_JSON=$(generate_post_json)


curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/networks
