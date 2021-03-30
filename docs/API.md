# API Reference Doc

###  Nodes
| Operation | HTTP Verb | Endpoint |
|-----------|-----------|----------|
| Get Peer List | `GET` | `/api/{group}/peerlist` |
| Get List Last Modified Date | `GET` | `/api/{group}/lastmodified` |
| Get Node Details | `GET` | `/api/{group}/nodes/{macaddress}` |
| Create Node | `POST` | `/api/{group}/nodes` |
| Uncordon Node | `POST` | `/api/{group}/nodes/{macaddress}/uncordon` |
| Check In Node | `POST` | `/api/{group}/nodes/{macaddress}/checkin` |
| Update Node | `POST` | `/api/{group}/nodes/{macaddress}` |
| Delete Node | `POST` | `/api/{group}/nodes/{macaddress}` |
| Get Group Nodes | `GET` | `/api/{group}/nodes` |
| Get All Nodes | `GET` | `/api/nodes` |
| Authenticate | `POST` | `/api/{group}/authenticate` |


### Groups
| Operation | HTTP Verb | Endpoint |
|-----------|-----------|----------|
|Get Groups | `GET` | `/api/groups` |
|Get Group Details | `GET` | `/api/group/{groupname}` |
|Get Number of Nodes in Group | `GET` | `/api/group/{groupname}/numnodes` |
|Create Group | `POST` | `/api/groups` |
|Update Group | `PUT` | `/api/groups/{groupname}` |
|Delete Group | `DELETE` | `/api/groups/{groupname}` |
|Create Access Key | `POST` |  `/api/groups/{groupname}/keys` |
|Get Access Key | `GET` | `/api/groups/{groupname}/keys` |
|Delete Access Key | `DELETE` | `/api/groups/{groupname}/keys/{keyname}` |

### Users (only used for interface admin user at this time)
| Operation | HTTP Verb | Endpoint |
|-----------|-----------|----------|
|Create Admin User| `POST` | `/users/createadmin` |
|Check for Admin User| `GET` | `/users/hasadmin` |
|Update User| `PUT` | `/users/{username}` |
|Delete User| `DELETE` | `/users/{username}` |
|Get User| `GET` | `/users/{username}` |
|Authenticate User | `POST` | `/users/authenticate` |

### Notes
* users API does not use `/api/` because of a weird bug. Will fix in future
  release.
* Only able to create Admin at this time. The "user" is only used by the [user
  interface](https://github.com/falconcat-inc/WireCat-UI) to authenticate the
  single admin user.

### Files
| Operation | HTTP Verb | Endpoint |
|-----------|-----------|----------|
| Get File | `GET` | `/meshclient/files/{filename}` |

---
## Example API CALLS

**Note About Token:** This is a configurable value stored under
`config/environments/dev.yaml` and can be changed before startup. It's a hack
for testing, just provides an easy way to authorize, and should be removed and
changed in the future.

#### Create a Group
```bash
curl --location --request POST 'localhost:8081/api/groups' \
--header 'Authorization: Bearer secretkey' \
--header 'Content-Type: application/json' \
--data-raw '{
    "addressrange": "10.70.0.0/16",
    "nameid": "skynet"
}'
```
#### Create a Key
```bash
curl --location --request POST 'localhost:8081/api/groups/skynet/keys' \
--header 'Authorization: Bearer secretkey' \
--header 'Content-Type: application/json' \
--data-raw '{
    "uses": 10
}'
```
#### Create a Node
```bash
curl --location --request POST 'localhost:8081/api/skynet/nodes' \
--header 'Content-Type: application/json' \
--header 'authorization: Bearer secretkey' \
--data-raw '{
    "endpoint": 100.200.100.200,
    "publickey": "aorijqalrik3ajflaqrdajhkr",
    "macaddress": "8c:90:b5:06:f1:d9",
    "password": "reallysecret",
    "localaddress": "172.16.16.1",
    "accesskey": "aA3bVG0rnItIRXDx",
    "listenport": 6400
}'
```
#### Get Groups
```bash
curl --location --request GET 'localhost:8081/api/groups' \
--header 'Authorization: Bearer secretkey' \
--header 'Content-Type: application/json' -s | jq
```
#### Get Group Nodes
```bash
curl --location --request GET 'localhost:8081/api/skynet/nodes' \
--header 'Authorization: Bearer secretkey' \
--header 'Content-Type: application/json' -s | jq
```
#### Update Node Settings
```bash
curl --location --request PUT 'localhost:8081/api/skynet/nodes/8c:90:b5:06:f1:d9' \
--header 'Content-Type: application/json' \
--header 'authorization: Bearer secretkey' \
--data-raw '{
    "name": "my-laptop"
}'
```
#### Delete a Node
```bash
curl --location --request DELETE 'localhost:8081/api/skynet/nodes/8c:90:b5:06:f1:d9' \
--header 'authorization: Bearer secretkey'
```

