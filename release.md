# Netmaker v0.24.0

## Whats New ✨

- IPv6 and Dual Stack Networks Support Across Platform
- Endpoint Detection Can Now Be Turned Off By Setting `ENDPOINT_DETECTION=false` On Server Config
- New SignUp Flow For Oauth Users, With Admin Approval Process.
- Added Failover Commands to nmctl

## What's Fixed/Improved 🛠

- Scalability Fixes around Mq connection, ACLs
- Fixed Zombie Node Logic To Avoid Choking On the Channel
- Fixed Egress Routes In Dual Stack Netmaker Overlay Networks
- Fixed Client Connectivity Metrics Data
- Fixed auto-relay with enrollment key
- Imporved Logic Around Oauth Sceret Management
- Improved Oauth Message Templates

## Known Issues 🐞

- Erratic Traffic Data In Metrics
- `netclient server leave` Leaves a Stale Node Record In At Least One Network When Part Of Multiple Networks, But Can Be Deleted From The UI.
- On Darwin Stale Egress Route Entries Remain On The Machine After Removing Egress Range Or Removing The Egress Server
