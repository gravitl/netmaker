#!/bin/bash

USERNAME="nme"
PASSWORD="testpass"

generate_post_json ()
{
  cat <<EOF
{
  "username": "$USERNAME",
  "password": "$PASSWORD"
}
EOF
}

POST_JSON=$(generate_post_json)

curl --max-time 5.0 -d "$POST_JSON" -H 'Content-Type: application/json' localhost:8081/users/createadmin

