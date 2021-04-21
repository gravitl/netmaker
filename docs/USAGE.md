# Usage

This guide covers advanced usage of Netmaker. If you are just looking to get started quickly, check out the Quick Start in the [README](../README.md).

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

## Client  Config

Client config files are stored under /etc/netclient  per network as /etc/netclient/netconfig-< network name >  
**server:**  
    address: The address:port of the server  
    accesskey: The acceess key used to sign up with the server  
  
**node:**  
    name: a displayname for the node, e.g. "mycomputer" 
    interface:  the network interface name, by default something like "nm-"  
    network: the netmaker network being attached to  
    password: the node's hashed password. Can be changed by putting a value in here and setting "postchanges" to "true"   
    macaddress: the mac address of the node  
    localaddress: the local network address   
    wgaddress: the wireguard private address  
    roamingoff: flag to update the IP address automatically based on network changes  
    islocal: whether or not this is a local or public network   
    allowedips: the allowedips addresses that other nodes will recieve  
    localrange: the local address range if it's a local network  
    postup: post up rules for gateway nodes  
    postdown: post down rules for gateway nodes  
    port: the wiregard port   
    keepalive: the default keepalive value between this and all other nodes  
    publickey: the public key other nodes will use to access this node   
    privatekey: the private key of the nodes (this field does nothing)  
    endpoint: the reachable endpoint of the node for routing, either local or public.  
    postchanges: either "true" or "false" (with quotes). If true, will post any changes you make to the remote server. 


## Non-Docker Installation

### MongoDB Setup
1.  Install MongoDB on your server. For Ubuntu: `sudo apt install -y mongodb`. For more advanced installation or other operating systems, see  the [MongoDB documentation](https://docs.mongodb.com/manual/administration/install-community/).

2. Create a user:
`mongo admin`
`db.createUser({ user: "mongoadmin" , pwd: "mongopass", roles: ["userAdminAnyDatabase", "dbAdminAnyDatabase", "readWriteAnyDatabase"]})`

### Server Setup
 1. **Run the install script:** sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.2/netmaker-server.sh | sh -
 2. Check status:  `sudo journalctl -u netmaker`
2. If any settings are incorrect such as host or mongo credentials, change them under /etc/netmaker/config/environments/ENV.yaml and then run `sudo systemctl restart netmaker`

### UI Setup
1. **Download UI asset files:** `sudo wget -O /usr/share/nginx/html/netmaker-ui.zip https://github.com/gravitl/netmaker-ui/releases/download/latest/netmaker-ui.zip`

2. **Unzip:** `sudo unzip /usr/share/nginx/html/netmaker-ui.zip -d /usr/share/nginx/html`

3. **Copy Config to Nginx:** `sudo cp /usr/share/nginx/html/nginx.conf /etc/nginx/conf.d/default.conf`

4. **Modify Default Config Path:** `sudo sed -i 's/root \/var\/www\/html/root \/usr\/share\/nginx\/html/g' /etc/nginx/sites-available/default`

5. **Change Backend URL:** `sudo sh -c 'BACKEND_URL=http://<YOUR BACKEND API URL>:PORT /usr/share/nginx/html/generate_config_js.sh >/usr/share/nginx/html/config.js'`

6. **Start Nginx:** `sudo systemctl start nginx`

### Agent  Setup

On each machine you would like to add to the network, do the following:

1. Confirm wireguard is installed: `sudo apt install wireguard-tools`
2. Confirm ipv4 forwarding is enabled: `sysctl -w net.ipv4.ip_forward=1`
3. Create a key or enable manual node signup at the network level
4. Run the install command generated by key create: `sudo curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/v0.2/netclient-install.sh | KEY=YOUR_TOKEN sh -`  
4.a. For subsequent installs, you can just run `sudo netclient -c install -t YOUR_TOKEN`
4.b. For offline installs, you can self-host a netclient file server on netmaker

This will install netclient@.service and netclient-YOUR_NET.timer in systemd, which will run periodically to call the netclient binary, which will check to see if there are any updates that it needs and update WireGuard appropriately.

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
