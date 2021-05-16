#!/bin/bash

USES=1000

generate_post_json ()
{
  cat <<EOF
{
  "uses": $USES
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8082/api/networks/skynet/keys
