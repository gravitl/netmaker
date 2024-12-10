# Netmaker v0.30.0

## Whats New ✨
- New ACLs and Tag Management System
- Managed DNS system (Linux)
- Simplified User Mgmt With Default Roles and Groups (Hidden away network roles)
- New Add a Node Flow for netclient and static wireguard files

## What's Fixed/Improved 🛠
- Metrics Data
- FailOver Stability Fixes
- Scalability Fixes

## Known Issues 🐞

- Adding Custom Private/Public Key For Remote Access Gw Clients Doesn't Get Propagated To Other Peers.
- IPv6 DNS Entries Are Not Working.
- Stale Peer On The Interface, When Forced Removed From Multiple Networks At Once.
- Can Still Ping The Domain Name Even When The DNS Toggle Is Switched Off.
- WireGuard DNS issue on most flavours of Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Remote Access Client (RAC) and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

