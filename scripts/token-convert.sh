#!/bin/bash

set -e

token=$1

token_json=$(echo $token | base64 -d)

api_addr=$(echo $token_json | jq -r '.apiconnstring')
network=$(echo $token_json | jq -r '.network')
key=$(echo $token_json | jq -r '.key')

echo ./netclient join -k $key -n $network --apiserver $api_addr 
