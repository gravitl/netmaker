#!/bin/ash

encrypt_password() {
  echo "${MQ_USERNAME}:${MQ_PASSWORD}" > /mosquitto/passwords.txt
  mosquitto_passwd -U /mosquitto/passwords.txt
}

main(){

 encrypt_password
 echo "Starting MQ..."
 # Run the main container command.
 /docker-entrypoint.sh
 /usr/sbin/mosquitto -c /mosquitto/config/mosquitto.conf

}

main "${@}"
