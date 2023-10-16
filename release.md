
# Netmaker v0.21.1

## Whats New
- Remote Access Client Session Management, refer users section in docs for more details
- Can now create generic DNS entries
- Upgrade client version to match server version from UI
- Moved PersistentKeepAlive setting from node to host level
## What's Fixed
- Extclients DNS now properly set from ingress dns value provided
- Allow Role Update of OAuth user
- Fixed zombie node issue
## known issues
- Windows installer does not install WireGuard
- netclient-gui will continously display error dialog if netmaker server is offline
- Mac IPv6 addresses/route issues
- Docker client can not re-join after complete deletion
- netclient-gui network tab blank after disconnect


