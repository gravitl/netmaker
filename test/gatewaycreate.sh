#!/bin/bash

generate_post_json ()
{
  cat <<EOF
{
  "rangestring": "172.31.0.0/16",
  "interface": "eth0"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" 3.86.23.0:8081/api/nodes/default/12:5a:ac:3f:03:2d

