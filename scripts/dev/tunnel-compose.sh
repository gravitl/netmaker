#!/usr/bin/env bash

SSH_HOST=$1
SCRIPT_DIR=$(dirname "$(realpath "$0")")

source "$SCRIPT_DIR/../netmaker.env"

# TODO read 1883 from SERVER_BROKER_ENDPOINT
# $API_PORT 8081
sshpass -p123123 ssh $SSH_HOST \
	-R 0.0.0.0:$API_PORT:localhost:$API_PORT \
	-L 1883:mq:1883 \
	-N -vv

	# TODO UDP fwd 3478
	#-R $STUN_PORT:localhost:$STUN_PORT
