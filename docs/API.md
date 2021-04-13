# API Reference Doc

###  Nodes
**Get Peer List:** "/api/{network}/peerlist", "GET"  
**Get List Last Modified Date:** "/api/{network}/lastmodified", "GET"  
**Get Node Details:** "/api/{network}/nodes/{macaddress}", "GET"  
**Create Node:** "/api/{network}/nodes", "POST"  
**Uncordon Node:** "/api/{network}/nodes/{macaddress}/uncordon", "POST"  
**Check In Node:** "/api/{network}/nodes/{macaddress}/checkin", "POST"  
**Update Node:** "/api/{network}/nodes/{macaddress}", "PUT"  
**Delete Node:** "/api/{network}/nodes/{macaddress}", "DELETE"  
**Get Network Nodes:** "/api/{network}/nodes", "GET" 
**Get All Nodes:** "/api/nodes", "GET"
**Authenticate:** "/api/{network}/authenticate", "POST"

 
### Networks
**Get Networks:** "/api/networks", "GET"  
**Get Network Details:** "/api/network/{networkname}", "GET"  
**Get Number of Nodes in Network:** "/api/network/{networkname}/numnodes", "GET"  
**Create Network:** "/api/networks", "POST"  
**Update Network:** "/api/networks/{networkname}", "PUT"  
**Delete Network:** "/api/networks/{networkname}", "DELETE"  

**Create Access Key:** "/api/networks/{networkname}/keys", "POST"  
**Get Access Key:** "/api/networks/{networkname}/keys", "GET"  
**Delete Access Key:** "/api/networks/{networkname}/keys/{keyname}", "DELETE" 

### Users (only used for interface admin user at this time)
**Create Admin User:** "/users/createadmin", "POST"  
**Check for  Admin User:** "/users/hasadmin", "GET"  
**Update User:** "/users/{username}", "PUT"  
**Delete User:** "/users/{username}", "DELETE"  
**Get User:** "/users/{username}", "GET"  
**Authenticate User:** "/users/authenticate", "POST"  

*note: users API does not use /api/ because of  a weird bug. Will fix in  future release.
**note: Only able to create Admin at this time. The "user" is only used by the [user interface](https://github.com/gravitl/netmaker-ui) to authenticate the  single  admin user.

### Files
**Get File:** "/meshclient/files/{filename}", "GET"  

## Example API CALLS

**Note About Token:** This is a configurable value stored under  config/environments/dev.yaml and can be changed before  startup. It's a hack for testing, just provides an easy way to authorize, and should be removed and changed in the future.

#### Create a Network
curl -d '{"addressrange":"10.70.0.0/16","netid":"skynet"}' -H "Authorization: Bearer secretkey" -H 'Content-Type: application/json' localhost:8081/api/networks

#### Create a Key
curl -d '{"uses":10}' -H "Authorization: Bearer secretkey" -H 'Content-Type: application/json' localhost:8081/api/networks localhost:8081/api/networks/skynet/keys

#### Create a Node
curl  -d  '{ "endpoint": 100.200.100.200, "publickey": aorijqalrik3ajflaqrdajhkr,"macaddress": "8c:90:b5:06:f1:d9","password": "reallysecret","localaddress": "172.16.16.1","accesskey": "aA3bVG0rnItIRXDx","listenport": 6400}' -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/skynet/nodes

#### Get Networks
curl -H "Authorization: Bearer secretkey" -H 'Content-Type: application/json' localhost:8081/api/networks | jq

#### Get Network Nodes
curl -H "Authorization: Bearer secretkey" -H 'Content-Type: application/json' localhost:8081/api/skynet/nodes | jq

#### Update Node Settings
curl -X "PUT" -d '{"name":"my-laptop"}' -H 'Content-Type: application/json' -H "authorization: Bearer secretkey" localhost:8081/api/skynet/nodes/8c:90:b5:06:f1:d9

#### Delete a Node
curl -X "DELETE" -H "authorization: Bearer secretkey" localhost:8081/api/skynet/nodes/8c:90:b5:06:f1:d9


