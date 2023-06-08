
# Netmaker v0.20.2

## whats new
- 
    
## whats fixed
- enrollment keys for non-admins 
- client version displayed correctly in UI
- upd hole punching improvments
- SSL fallback to letsencrypt
- permission handling for non-admin users


## known issues
- Migration causes a listen port of 0 for some upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
