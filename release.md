# Netmaker v0.18.6

## whats new
- no new features
    
## whats fixed
- a few ext client/ingress issues
  - viewing addresses (UI)
  - when deleting an ingress gateway, ext clients are now removed from peers immediately
  - ext client peers should be populated immediately after creation
  - ext clients no longer reset public key when disabled/enabled
  - can delete an ingress without clients
- removed unnecessary host update
- host nat type is now collected from clients
- fix peer update issue where caclulation was happening to frequently
- nm-quick && nm-upgrade 
- EMQX image change && api routes

## known issues
- Caddy does not handle netmaker exporter well for EE
- Migration causes a listen port of 0 for some upgraded hosts
- Docker clients can not re-join after deletion
- Innacurate Ext Client Metrics 
- Issue with Mac + IPv6 addressing
- Nodes on same local network may not always connect
- List populates egress ranges twice
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
- No internet gateways/default routes
