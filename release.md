
# Netmaker v0.20.4

## Whats New
- Moved to new licensing server for self-hosted
- STUN removed from netmaker server to improve memory performance
- Added DB caching to drastically reduce read/writes from disk

## What's Fixed
- Major memory leak resolved due to STUN
- Issues with netclient ports on daemon restart
- Windows GUI unable to find netclient backend
- Major scalability fixes - Can now scale to hundreds of hosts with low resources
- Resolved ACL panic
- Reverted blocking creation of Ingress with NAT
     
## known issues
- netclient-gui (windows) will display an erroneous error dialog when joining a network (can be ignored)
- netclient-gui will continously display error dialog if netmaker server is offline
- Incorrect metrics against ext clients
- Host ListenPorts set to 0 after migration from 0.17.1 -> 0.20.4
- Mac IPv6 addresses/route issues
- Docker client can not re-join after complete deletion
- netclient-gui network tab blank after disconnect


