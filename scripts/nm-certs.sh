#!/bin/bash

CONFIG_FILE=netmaker.env

source $CONFIG_FILE

# TODO check $DOMAIN, $EMAIL
# TODO support EE domains

wget -O https://github.com/zerossl/zerossl-bot/raw/master/zerossl-bot.sh
chmod +x zerossl-bot.sh

./zerossl-bot.sh certonly --standalone \
	-m "$EMAIL" \
	-d "stun.nm.$DOMAIN" \
	-d "broker.nm.$DOMAIN" \
	-d "dashboard.nm.$DOMAIN" \
	-d "api.nm.$DOMAIN"
