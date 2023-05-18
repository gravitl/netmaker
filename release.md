
# Netmaker v0.20.0

## whats new
- New UI
- revamped compose-files and install scripts
- TURN
    
## whats fixed
- Caddy does not handle netmaker exporter well for EE

## known issues
- Migration causes a listen port of 0 for some upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
