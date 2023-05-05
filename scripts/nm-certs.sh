#!/bin/bash

CONFIG_FILE=netmaker.env
source $CONFIG_FILE
CERT_DIR=/etc/letsencrypt/live/stun.$DOMAIN/

echo "Setting up SSL certificates..."

# TODO check $DOMAIN, $EMAIL
# TODO support EE domains

wget -qO /root/zerossl-bot.sh "https://github.com/zerossl/zerossl-bot/raw/master/zerossl-bot.sh"
chmod +x /root/zerossl-bot.sh

RESTART_CADDY=false
if [ -n "$(docker ps | grep caddy)" ]; then
	echo "Caddy is running, stopping for now..."
	RESTART_CADDY=true
	docker-compose -f /root/docker-compose.yml stop caddy
fi

# request certs
./zerossl-bot.sh certonly --standalone \
	-m "$EMAIL" \
	-d "stun.$DOMAIN" \
	-d "broker.$DOMAIN" \
	-d "dashboard.$DOMAIN" \
	-d "api.$DOMAIN"

# TODO fallback to letsencrypt

# check if successful
if [ ! -f "$CERT_DIR"/fullchain.pem ]; then
	echo "SSL certificates failed"
	exit 1
fi

# copy for mounting
cp "$CERT_DIR"/fullchain.pem /root
cp "$CERT_DIR"/privkey.pem /root

echo "SSL certificates ready"

if [ "$RESTART_CADDY" = true ]; then
	echo "Starting Caddy..."
	docker-compose -f /root/docker-compose.yml start caddy
fi
