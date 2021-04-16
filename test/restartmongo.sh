#!/bin/bash

sudo docker kill mongodb
sudo docker rm mongodb
sudo docker volume rm mongovol

docker volume create mongovol && docker run -d --name mongodb -v mongovol:/data/db --network host -e MONGO_INITDB_ROOT_USERNAME=mongoadmin -e MONGO_INITDB_ROOT_PASSWORD=mongopass mongo --bind_ip 0.0.0.0
