# API Reference Doc

## GENERAL

Most actions that can be performed via API can be performed via UI. We recommend managing your networks using our official netmaker-ui project. That said, Netmaker is API based, and all functions can also be achieved via API calls. If you feel the need to work with Netmaker via API, we've provided some documentation below to help guide you.
 
#### Authentication
 In general, API calls must be authenticated via a header of  the format  `-H "Authorization: Bearer <YOUR_SECRET_KEY>"` There are two methods of obtaining YOUR_SECRET_KEY:
1. Using the masterkey. By default, this value is "secret key," but you should change this on your instance and keep it secure. This value can be set via env var at startup or in a config file (config/environments/< env >.yaml). See the [getting started](./GETTING_STARTED.md) documentation for more details.
2. Using a JWT recieved for a node. This  can be retrieved by calling the `/api/nodes/<network>/authenticate` endpoint, as documented below.

#### Format 
In general, requests will take the format of `curl -H "Authorization: Bearer <YOUR_SECRET_KEY>" -H 'Content-Type: application/json' localhost:8081/api/path/to/endpoint`

## NETWORKS

**Get All Networks:** `/api/networks`, `GET` 
  
**Create Network:** `/api/network`, `POST` 
  
**Get Network:** `/api/networks/{network id}`, `GET`  
  
**Update Network:** `/api/networks/{network id}`, `PUT`  
  
**Delete Network:** `/api/networks/{network id}`, `DELETE`  
  
**Cycle PublicKeys on all Nodes:** `/api/networks/{network id}/keyupdate`, `POST`  
  
  
### Network  API Call Examples
  
  
**Get All Networks:** `curl -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks | jq`

**Create Network:** `curl -d '{"addressrange":"10.70.0.0/16","netid":"skynet"}' -H "Authorization: Bearer YOUR_SECRET_KEY" -H 'Content-Type: application/json' localhost:8081/api/networks`

**Get Network:** `curl -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks/skynet | jq`

**Update Network:** `curl -X PUT -d '{"displayname":"my-house"}' -H "Authorization: Bearer YOUR_SECRET_KEY" -H 'Content-Type: application/json' localhost:8081/api/networks/skynet`

**Delete Network:** `curl -X DELETE -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks/skynet`

**Cycle PublicKeys on all Nodes:** `curl -X POST -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks/skynet/keyupdate`

## ACCESS KEYS

**Get All Keys:** `/api/networks/{network id}/keys`, `GET` 
  
**Create Key:** `/api/networks/{network id}/keys`, `GET` 
  
**Delete Key:** `/api/networks/{network id}/keys/{keyname}`, `DELETE` 
  
  
### Access Key API Call Examples
  
   
**Get All Keys:** `curl -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks/skynet/keys | jq`
  
**Create Key:** `curl -d '{"uses":10,"name":"mykey"}' -H "Authorization: Bearer YOUR_SECRET_KEY" -H 'Content-Type: application/json' localhost:8081/api/networks/skynet/keys`
  
**Delete Key:** `curl -X DELETE -H "Authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/networks/skynet/keys/mykey`
  
    
## NODES (COMPUTERS)
  
  
**Get All Nodes:** `/api/nodes`, `GET` 
  
**Get Network Nodes:** `/api/nodes/{network id}`, `GET` 
  
**Create Node:** `/api/nodes/{network id}`, `POST`  
  
**Get Node:** `/api/nodes/{network id}/{macaddress}`, `GET`  
  
**Update Node:** `/api/nodes/{network id}/{macaddress}`, `PUT`  
  
**Delete Node:** `/api/nodes/{network id}/{macaddress}`, `DELETE`  
  
**Check In Node:** `/api/nodes/{network id}/{macaddress}/checkin`, `POST`  
  
**Create a Gateway:** `/api/nodes/{network id}/{macaddress}/creategateway`, `POST`  
  
**Delete a Gateway:** `/api/nodes/{network id}/{macaddress}/deletegateway`, `DELETE`  
  
**Uncordon (Approve) a Pending Node:** `/api/nodes/{network id}/{macaddress}/uncordon`, `POST`  
  
**Get Last Modified Date (Last Modified Node in Network):** `/api/nodes/adm/{network id}/lastmodified`, `GET`  
  
**Authenticate:** `/api/nodes/adm/{network id}/authenticate`, `POST`  
  
  
### Example Node API Calls
  
  
**Get All Nodes:**`curl -H "Authorization: Bearer YOUR_SECRET_KEY" http://localhost:8081/api/nodes | jq`
  
**Get Network Nodes:** `curl -H "Authorization: Bearer YOUR_SECRET_KEY" http://localhost:8081/api/nodes/skynet | jq`
    
**Create Node:** `curl  -d  '{ "endpoint": 100.200.100.200, "publickey": aorijqalrik3ajflaqrdajhkr,"macaddress": "8c:90:b5:06:f1:d9","password": "reallysecret","localaddress": "172.16.16.1","accesskey": "aA3bVG0rnItIRXDx","listenport": 6400}' -H 'Content-Type: application/json' -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/skynet`
    
**Get Node:** `curl -H "Authorization: Bearer YOUR_SECRET_KEY" http://localhost:8081/api/nodes/skynet/{macaddress} | jq`  
  
**Update Node:** `curl -X PUT -d '{"name":"laptop1"}' -H 'Content-Type: application/json' -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/skynet/8c:90:b5:06:f1:d9`
  
**Delete Node:** `curl -X DELETE -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/skynet/nodes/8c:90:b5:06:f1:d9`
  
**Create a Gateway:** `curl  -d  '{ "rangestring": "172.31.0.0/16", "interface": "eth0"}' -H 'Content-Type: application/json' -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/skynet/8c:90:b5:06:f1:d9/creategateway`
  
**Delete a Gateway:** `curl -X DELETE -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/skynet/8c:90:b5:06:f1:d9/deletegateway`
  
**Approve a Pending Node:** `curl -X POST -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/skynet/8c:90:b5:06:f1:d9/approve`
  
**Get Last Modified Date (Last Modified Node in Network):** `curl -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/nodes/adm/skynet/lastmodified`

**Authenticate:** `curl -d  '{"macaddress": "8c:90:b5:06:f1:d9", "password": "YOUR_PASSWORD"}' -H 'Content-Type: application/json' localhost:8081/api/nodes/adm/skynet/authenticate`
  
  
## USERS
  
  
**Note:** Only able to create Admin user at this time. The "user" is only used by the [user interface](https://github.com/gravitl/netmaker-ui) to authenticate the  single  admin user.

**Get User:** `/api/users/{username}`, `GET`  
  
**Update User:** `/api/users/{username}`, `PUT`  
  
**Delete User:** `/api/users/{username}`, `DELETE`  
  
**Check for Admin User:** `/api/users/adm/hasadmin`, `GET` 
  
**Create Admin User:** `/api/users/adm/createadmin`, `POST` 
  
**Authenticate:** `/api/users/adm/authenticate`, `POST` 
  
  
### Example User API Calls

  
**Get User:**`curl -H "Authorization: Bearer YOUR_SECRET_KEY" http://localhost:8081/api/users/{username} | jq`

**Update User:** `curl -X PUT -d '{"password":"noonewillguessthis"}' -H 'Content-Type: application/json' -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/users/{username}`
  
**Delete User:** `curl -X DELETE -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/users/{username}`
  
**Check for Admin User:**`curl -H "Authorization: Bearer YOUR_SECRET_KEY" http://localhost:8081/api/users/adm/hasadmin`
  
**Create Admin User:** `curl -d '{ "username": "smartguy", "password": "YOUR_PASS"}' -H 'Content-Type: application/json' -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/users/adm/createadmin`
   
**Authenticate:** `curl -d  '{"username": "smartguy", "password": "YOUR_PASS"}' -H 'Content-Type: application/json' localhost:8081/api/nodes/adm/skynet/authenticate`
  
## SERVER MGMT

The Server Mgmt. API allows you to add and remove the server from networks.

**Add to Network:** `/api/server/addnetwork/{network id}`, `POST`  
**Remove from Network:** `/api/server/removenetwork/{network id}`, `DELETE`  

**Add to Network:**  `curl -X POST -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/server/addnetwork/{network id}`

**Remove from Network:** `curl -X DELETE -H "authorization: Bearer YOUR_SECRET_KEY" localhost:8081/api/server/removenetwork/{network id}`

## FILE SERVER
**Get File:** `/meshclient/files/{filename}`, `GET`
  
**Example:**  `curl localhost:8081/meshclient/files/meshclient`
