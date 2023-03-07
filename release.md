# Netmaker v0.18.3

## **Do not attempt upgrade from 0.17.x quite yet**

## whats new
- Enrollment Keys, give the ability for an admin to enroll clients into multiple networks, can be unlimited, time, or usage based
- EMQX broker support and better MQTT support in general
  - Now you must specify BROKER_ENDPOINT
  - Also specify SERVER_BROKER_ENDPOINT, if not provided server will connect to broker over BROKER_ENDPOINT
  - Thsi gives ability for user to specify any broker endpoint and use any protocal on clients desired, such as, `mqtts://mybroker.com:8083`
    (we will still default to wss)
    
## whats fixed
- Fixed default ACL behavior, should work as expected
- Peer calculations enhancement
- main routines share a context and docker stop/ctrl+c give expected results now
- Github workflow edits
- Removed Deprecated Local Network Range from client + server

## known issues
- EnrollmentKeys may not function as intended in an HA setup
- If a host does not receive a message to delete a node, it could become orphaned and un-deletable
- Network interface routes may be removed after sometime/unintended network update
- Upgrade script does not handle clients
- Caddy does not handle netmaker exporter well for EE
- Incorrect latency on metrics (EE)
- Swagger docs not up to date
