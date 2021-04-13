#!/bin/bash

NAME="skynet"
ADDRESSRANGE="10.71.0.0/16"

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
