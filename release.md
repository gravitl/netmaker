# Netmaker v0.18.4

## **Wait till out of pre-release to fully upgrade**

## whats new
- Forced node deletions, if a host doesn't not receive message to delete a node, you can forcefully remove it by deleting it twice from UI/CLI  
  - Allows user to remove orpahned Nodes + Hosts easier
- EMQX ACLs, if using EMQX as broker, ACLs per host will be created, enhancing security around messages
- You can now create ext clients with your own public key, but this feature will not be represented on current UI (new UI on the horizon)
- STUN is now represented as a list including your NM server + 2 we are hosting + 2 of googles (clients will only use 2) for better NAT detection
  - you specify which STUN servers to use with STUN_LIST env variable
    
## whats fixed
- More Peer calculation improvements
- JSON output on list commands for `nmctl`
- Upgrade script
- Ports set from server for Hosts on register/join are actually used
- **CLients**
  - More efficient Windows daemon handling
  - Better peer route setting on clients
  - Some commands involving the message queue on client have been fixed
  - NFTables masquerading issue
  - Some logging has been adjusted
  - Migrations on Linux work for 0.17.x - 0.18.3
  - EnrollmentKEys in an HA setup should function fine now
  - Registration by enrollment key on client GUI

## known issues
- Network interface routes may be removed after sometime/unintended network update
- Caddy does not handle netmaker exporter well for EE
- Incorrect latency on metrics (EE)
- Swagger docs not up to date
- Lengthy delay when you create an ext client
- issues connecting over IPv6 on Macs
- Nodes on same local network may not always connect
- Netclient GUI shows egress range(s) twice
- DNS entries are not sent after registration with EnrollmentKeys
- If you do NOT set STUN_LIST on server, it could lead to strange behavior on client
