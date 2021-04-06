# Getting Started (Simple Setup)
### Server Setup
 1. Get yourself a linux server and make sure it has a public IP.
 2. Deploy MongoDB `docker volume create mongovol && docker run -d --name mongodb -v mongovol:/data/db --network host -e MONGO_INITDB_ROOT_USERNAME=mongoadmin -e MONGO_INITDB_ROOT_PASSWORD=mongopass mongo --bind_ip 0.0.0.0 `
 3. Pull this repo: `git clone https://github.com/gravitl/netmaker.git`
 4. Switch to the directory and source the default env vars `cd netmaker && source defaultvars.sh`
 5. Run the server: `go run ./`
### Optional (For  Testing):  Create Groups and Nodes
 
 1. Create Group: `./test/groupcreate.sh`
 2. Create Key: `./test/keycreate.sh` (save the response for step 3)
 3. Open ./test/nodescreate.sh and replace ACCESSKEY with value from #2
 4. Create Nodes: `./test/nodescreate.sh`
 5. Check to see if nodes were created: `curl -H "authorization: Bearer secretkey" localhost:8081/api/skynet/nodes | jq`
### UI Setup
Please see [this repo](https://github.com/gravitl/netmaker-ui)  for instructions on setting up your UI.

### Agent  Setup

On each machine you would like to add to the network, do the following:

1. Confirm wireguard is installed: `sudo apt install wireguard-tools`
2. Confirm ipv4 forwarding is enabled: `sysctl -w net.ipv4.ip_forward=1`
3. Create a key or enable manual node signup at the group level
4. Get the binary: `sudo wget 52.55.6.84:8081/meshclient/files/meshclient`
5. Make it executable: `sudo chmod +x meshclient`
6. Run the install command: `sudo ./meshclient -c install -g <group name> -s <server:port> -k <key value>`

This will install netclient.service and netclient.timer in systemd, which will run periodically to call the netclient binary, which will check to see if there are any updates that it needs and update WireGuard appropriately.

## BUILDING
**Protoc command for GRPC Compilation:** 

    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/node.proto

**Build binary:**   `go build ./` 

