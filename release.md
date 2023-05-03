# Netmaker v0.19.0

## whats new
- internet gateways (0.0.0.0/0) for egress
- deprecated editing of network parameters
- allow extra ips for extclient (not enabled in UI)
    
## whats fixed
- nm-quick - determine lastest version from releases
- wireguard public/private key rotation
- ee-license checks

## known issues
- Caddy does not handle netmaker exporter well for EE
- Migration causes a listen port of 0 for some upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
