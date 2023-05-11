#!/bin/bash

CONFIG_FILE=netmaker.env
SCRIPT_DIR=$(dirname "$(realpath "$0")")

# get and check the config
if [ ! -f "$SCRIPT_DIR/$CONFIG_FILE" ]; then
	echo "Config file missing"
	exit 1
fi
source "$SCRIPT_DIR/$CONFIG_FILE"
if [ -z "$NM_DOMAIN" ] || [ -z "$NM_EMAIL" ]; then
	echo "Config not valid"
	exit 1
fi

# TODO make sure this doesnt break, parse `certbot certificates` if yes
CERT_DIR="$SCRIPT_DIR/letsencrypt/live/stun.$NM_DOMAIN"

echo "Setting up SSL certificates..."

# preserve the env state
RESTART_CADDY=false
if [ -n "$(docker ps | grep caddy)" ]; then
	echo "Caddy is running, stopping for now..."
	RESTART_CADDY=true
	docker-compose -f /root/docker-compose.yml stop caddy
fi

CERTBOT_PARAMS=$(cat <<EOF
certonly --standalone \
	--non-interactive --agree-tos \
	-m "$NM_EMAIL" \
	-d "stun.$NM_DOMAIN" \
	-d "api.$NM_DOMAIN" \
	-d "broker.$NM_DOMAIN" \
	-d "dashboard.$NM_DOMAIN" \
	-d "turn.$NM_DOMAIN" \
	-d "turnapi.$NM_DOMAIN" \
	-d "netmaker-exporter.$NM_DOMAIN" \
	-d "grafana.$NM_DOMAIN" \
	-d "prometheus.$NM_DOMAIN"
EOF
)

# generate an entrypoint for zerossl-certbot
cat <<EOF >"$SCRIPT_DIR/certbot-entry.sh"
#!/bin/sh
# deps
apk add bash curl
# zerossl
wget -qO zerossl-bot.sh "https://github.com/zerossl/zerossl-bot/raw/master/zerossl-bot.sh"
chmod +x zerossl-bot.sh
# request the certs
./zerossl-bot.sh "$CERTBOT_PARAMS"
EOF
chmod +x certbot-entry.sh

# request certs
sudo docker run -it --rm --name certbot \
	-p 80:80 -p 443:443 \
	-v "$SCRIPT_DIR/certbot-entry.sh:/opt/certbot/certbot-entry.sh" \
	-v "$SCRIPT_DIR/letsencrypt:/etc/letsencrypt" \
	--entrypoint "/opt/certbot/certbot-entry.sh" \
	certbot/certbot

# clean up TODO enable
#rm "$SCRIPT_DIR/certbot-entry.sh"

# check if successful
if [ ! -f "$CERT_DIR"/fullchain.pem ]; then
	# fallback to letsencrypt-certbot
	sudo docker run -it --rm --name certbot \
		-p 80:80 -p 443:443 \
		-v "$SCRIPT_DIR/letsencrypt:/etc/letsencrypt" \
		--entrypoint "/opt/certbot/certbot-entry.sh" \
		certbot/certbot "$CERTBOT_PARAMS"
	if [ ! -f "$CERT_DIR"/fullchain.pem ]; then
		echo "Missing file: $CERT_DIR/fullchain.pem"
		echo "SSL certificates failed"
		exit 1
	fi
fi

# copy for mounting
mkdir -p certs
cp -L "$CERT_DIR/fullchain.pem" /root/certs/fullchain.pem
cp -L "$CERT_DIR/privkey.pem" /root/certs/privkey.pem

echo "SSL certificates ready"

# preserve the env state
if [ "$RESTART_CADDY" = true ]; then
	echo "Starting Caddy..."
	docker-compose -f /root/docker-compose.yml start caddy
fi

# install crontab
ln -sfn "$SCRIPT_DIR"/nm-certs.sh /etc/cron.monthly/nm-certs.sh
