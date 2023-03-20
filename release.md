# Netmaker v0.18.4

## **Wait till out of pre-release to fully upgrade**

## whats new
- Logic for ext client ACLs (not really usable until new UI is finished)
- Default proxy mode, enables users to determine if all Hosts should have proxy enabled/disabled/auto by default
  - specify with DEFAULT_PROXY_MODE="on/off/auto" 
    
## whats fixed
- Proxy Peer calculation improvements
- DNS is populated correctly after registration by enrollment key
- Migrate is functional for Windows/Mac **note** Ports may be set to 0 after an upgrade, can be adjusted via UI to fix
- Interface data is sent on netclient register
- Upgrade script
- Latency issue with Node <-> Node Metrics
- Ports set from server for Hosts on register/join are actually used

## known issues
- Caddy does not handle netmaker exporter well for EE
- Migration causes a listen port of 0 for upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
