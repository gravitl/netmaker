#!/bin/bash
sudo ip link del wc-skynet

curl -X DELETE -H "Authorization: Bearer secretkey" -H 'Content-Type: application/json' localhost:8081/api/skynet/nodes/8c:89:a5:03:f0:d7 | jq

sudo cp /root/.wcconfig.bkup /root/.wcconfig
sudo rm /root/.wctoken
sudo go run ./main.go remove

sudo wg show
