# Netmaker v0.24.0

## Whats New ✨

- Added Failover Commands to nmctl
- IPv6 and Dual Stack Networks Support Across Platform
- Endpoint Detection Can Now Be Turned Off By Setting `ENDPOINT_DETECTION=false` On Server Config

## What's Fixed/Improved 🛠

- Scalability Fixes around Mq connection, ACLs
- Fixed Zombie Node Logic To Avoid Choking On the Channel
- Fixed Egress Routes In Dual Stack Netmaker Overlay Networks
- Fixed Client Connectivity Metrics Data

## Known Issues 🐞

- Erratic Traffic Data In Metrics
- `netclient server leave` Leaves a Stale Node Record In At Least One Network When Part Of Multiple Networks, But Can Be Deleted From The UI.
- On Darwin Stale Egress Route Entries Remain On The Machine After Removing Egress Range Or Removing The Egress Server
