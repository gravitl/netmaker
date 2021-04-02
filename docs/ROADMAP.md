# FEATURE ROADMAP

### 0.1
**Server:**
 - [x] Create Groups (virtual networks)
 - [x] Allow default settings for nodes from groups
 - [x] Admin/Superuser key
 - [x] Create multiuse keys for node signup
 - [x] JWT-based auth for post-signup
 - [x] CRUD for groups
 - [x] CRUD for nodes
 - [x] Track all important info about node for networking (port, endpoints, pub key, etc)
 - [x] Timestamps for determining if nodes need updates
 
**Agent:**
 - [x] Self-installer
 - [x] Determine default settings w/o user input
 - [x] Systemd Service + timer
 - [x] Check-in functionality to retrieve updates from server
 - [x] Maintain list of up-to-date peers
 - [x] Update WG interface
 - [x] Config file for modifying node 

### 0.2
- [ ] Separate out README into DOCS folder with the following:
	- [ ] API Docs
	- [ ] Getting Started
	- [ ] Advanced Usage
	- [ ] Contributing
	- [ ] Roadmap
	- [ ] Troubleshooting

**Server:**
 - [ ] Allow tracking multiple groups per node
 - [ ] Configure Check-in thresholds
 - [ ] Separate sign-up endpoint to allow VPN-only comms after joining network
 - [ ] Swagger Docs
 - [ ] Build Out README
 - [ ] Encode Server, Port, and Group into Keys
 - [ ] Switch to Unique ID for nodes instead of MacAddress
 - [ ] Public Key refresh
 - [ ] Enable  ipv6 addresses
 - [ ] Have a "default" group created at startup
 
**Agent:**
 - [ ] Test / get working on multiple linux platforms
 - [ ] Set private DNS via etc hosts (node name + ip). Make it optional flag on agent.
 - [ ] Decode Server, Port, and Group from Key
 - [ ] Service ID / unit file for SystemD Service
 - [ ] Allow multiple interfaces
 - [ ] Use "Check in interval" from server
 - [ ] Pre-req check on machine (wg, port forwarding)
 - [ ]  Enable  ipv6 addresses

### 0.3
**Server:**
 - [ ] Swagger Docs
 - [ ] Group/Node labels
 - [ ] "Read Only" mode for nodes (can't update their settings centrally, only read)
 - [ ] "No-GUI mode:" Similar to existing, just do more e2e testing and make sure flow makes sense
 - [ ] Let users set prefixes (node, interface)
 
**Agent:**
 - [ ] Do system calls instead of direct commands
 - [ ] Add a prompt for easy setup

### 0.4
**Server:**
 - [ ] Private  DNS
 - [ ] UDP Hole-Punching (via WGSD: https://github.com/jwhited/wgsd )
 - [ ] "Read Only" mode for nodes (can't update their settings centrally, only read)
 
**Agent:**
 - [ ] Do system calls instead of direct commands [this repo](https://github.com/gravitl/netmaker-ui)
 - [ ] Add a prompt for easy setup
 - [ ] Make it work as a sidecar container!!!

### 0.5
**Server:**
 - [ ] Multi-user support
 - [ ] Oauth
 - [ ] public key cycling
 
### Future Considerations
**Server:**
 - [ ] Switch to distributed protocol (RAFT, Kademlia) instead of central server
 - [ ] Load balance / fault tolerant server
 - [ ] Change DB / make more scaleable (SQL?)
 - [ ] Redis
 - [ ] Group/Node labels
 
**Agent:**
 - [ ] userspace via Docker or Golang
 - [ ] MacOS support
 - [ ] Windows support
 - [ ] Certificate-based authentication
