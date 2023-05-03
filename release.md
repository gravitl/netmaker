# Netmaker v0.19.1

## whats new
- 
    
## whats fixed
- status code for exceeding free tier limits  

## known issues
- Caddy does not handle netmaker exporter well for EE
- Migration causes a listen port of 0 for some upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
