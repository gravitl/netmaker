# Usage

This guide covers advanced usage of Netmaker. If you are just looking to get started quickly, check out the Quick Start in the [README](../README.md).

## Index

 - Config
	 - Server Config
	 - Agent Config
	 - UI Config
 - Creating Your  Network
	 - Creating Networks
	 - Creating Keys
	 - Creating Nodes
	 - Managing Your Network
 - Cleaning up
 - Non-Docker Installation
 - Building
 - Testing 

## Server Config
Netmaker settings can be set via Environment Variables or Config file. There are also a couple of runtime arguments that can optionally be set.

### Environment Variables
**APP_ENV**: default=dev. Determines which environment file to use. Will look under config/environments/APP_ENV.yaml. For instance, you can  have different environments  for dev,  test, and prod,  and store different settinggs  accordingly.
**GRPC_PORT**: default=50051. The port for GRPC (node/client) communications
**API_PORT**: default=8081. The port for API and UI communications
**MASTER_KEY**: default=secretkey. The skeleton key used for authenticating with server as administrator.

MongoDB Connection Env Vars:
**MONGO_USER**:default=admin
**MONGO_HOST**:default=password
**MONGO_PASS**:default=localhost
**MONGO_PORTS**:default=27017
**MONGO_OPTS**:default=/?authSource=admin

**BACKEND_URL**: default=nil. The address of the server. Used for setting token values  for client/nodes. If not set, will run a command to retrieve the server URL.

###   Config File
Stored as config/environments/*.yaml. Default used is dev.yaml

**server**:
  - **host:** "localhost" (reachable address of this server, overriden by BACKEND_URL)
  - **apiport:** "8081" (api port, overriden  by API_PORT)
  - **grpcport**: "50051" (grpc port, overridden by GRPC_PORT)
  - **masterkey**: "secretkey" (administrator server API key, overridden by MASTER_KEY)
  - **allowedorigin**: "*" (CORS policy  for requests)
  - **restbackend**: true (Runs the REST server)
  - **agentbackend**: true (Runs the GRPC server)
  - **defaultnetname**: "default" (name for the default  network)
  - **defaultnetrange**: "10.10.10.0/24" (range for the default network)
  - **createdefault**: true (Flag for creating the default network)
  
**mongoconn**: (see ENV values above for explanation.  ENV values override.)
  - **user**: "mongoadmin"
  - **pass**: "mongopass"
  - **host**: "localhost"
  - **port**: "27017"
  - **opts**: '/?authSource=admin'

### Runtime Args

**clientmode**: (default=on) E.x.: `sudo netmaker --clientmode=off` Run the Server as a client (node) as well.
**defaultnet**:  (default=on) E.x.: `sudo netmaker --defaultnet=off` Create a default network on startup.

### Running the Backend Components on Different Machines
HTTP, GRPC, MongoDB

### Non-Docker Installation

### Server Setup
 1. Get yourself a linux server and make sure it has a public IP.
 2. Deploy MongoDB `docker volume create mongovol && docker run -d --name mongodb -v mongovol:/data/db --network host -e MONGO_INITDB_ROOT_USERNAME=mongoadmin -e MONGO_INITDB_ROOT_PASSWORD=mongopass mongo --bind_ip 0.0.0.0 `
 3. Pull this repo: `git clone https://github.com/gravitl/netmaker.git`
 4. Switch to the directory and source the default env vars `cd netmaker && source defaultvars.sh`
 5. Run the server: `go run ./`
### Optional (For  Testing):  Create Networks and Nodes
 
 1. Create Network: `./test/networkcreate.sh`
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
3. Create a key or enable manual node signup at the network level
4. Get the binary: `sudo wget 52.55.6.84:8081/meshclient/files/meshclient`
5. Make it executable: `sudo chmod +x meshclient`
6. Run the install command: `sudo ./meshclient -c install -g <network name> -s <server:port> -k <key value>`

This will install netclient.service and netclient.timer in systemd, which will run periodically to call the netclient binary, which will check to see if there are any updates that it needs and update WireGuard appropriately.

## BUILDING
**Back End Compilation** 
The backend can be compiled by running "go build" from the  root of the repository,  which will create an executable named "netmaker." 

**Client Compilation**
Similarly, "go build" can be run from the netclient directory to produce a netclient executable.

**Protoc command for GRPC Compilation:** 
Whenever making changes to grpc/node.proto, you will need to recompile the grpc. This can be achieved by running the following command from the root of the repository.

    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/node.proto

**Build binary:**   `go build ./` 


## TESTING

**Unit Testing**
When making changes to Netmaker, you may wish to create nodes, networks, or keys for testing. Bash scripts have been created under the "test" directory (*.sh) which run curl commands that generate sample nodes, networks, and keys that can be used for testing purposes.

**Integration Testing**
Similarly, several go  scripts have been created under the test directory (*.go) to test out changes to the code base.  These will be run automatically when PR's are submitted but can also be run manually using "go test."

