
# Netmaker v0.20.5

## Whats New
- FreeBSD 13/14 specific binaries
- Whitelabelling capabilities

## What's Fixed
- Fixes for FreeBSD
- Mac installer installs WireGuard
- ACL rendering on UI
- Updating Endpoint IP from UI
     
## known issues
- Windows installer does not install WireGuard
- netclient-gui (windows) will display an erroneous error dialog when joining a network (can be ignored)
- netclient-gui will continously display error dialog if netmaker server is offline
- Incorrect metrics against ext clients
- Host ListenPorts set to 0 after migration from 0.17.1 -> 0.20.5
- Mac IPv6 addresses/route issues
- Docker client can not re-join after complete deletion
- netclient-gui network tab blank after disconnect


