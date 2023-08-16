
# Netmaker v0.20.6

## Whats New
- Sync clients with server state from UI

## What's Fixed
- Upgrade Process from v0.17.1 to latest version can be now done seamlessly, please refer docs
- Expired nodes clean up is handled correctly now
- Ext client config generation fixed for ipv6 endpoints
- installation process will only generate certs required for required Domains based on CE or EE
- support for ARM machines on install script
     
## known issues
- Windows installer does not install WireGuard
- netclient-gui will continously display error dialog if netmaker server is offline
- Mac IPv6 addresses/route issues
- Docker client can not re-join after complete deletion
- netclient-gui network tab blank after disconnect


