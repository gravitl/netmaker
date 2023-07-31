
# Netmaker v0.20.5

## Whats New
- Extclient Acls
- Force delete host along with all the associated nodes

## What's Fixed
- Deprecated Proxy
- Solved Race condition for multiple nodes joining network at same time
- Node dns toggle
- Simplified Firewall rules for added stability
     
## known issues
- Windows installer does not install WireGuard
- netclient-gui will continously display error dialog if netmaker server is offline
- Incorrect metrics against ext clients
- Host ListenPorts set to 0 after migration from 0.17.1 -> 0.20.5
- Mac IPv6 addresses/route issues
- Docker client can not re-join after complete deletion
- netclient-gui network tab blank after disconnect


