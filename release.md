# Netmaker v0.26.0

## Whats New ✨
- Advanced User Management with Network Roles and Groups
- User Invitation via Email and Magic Links

## What's Fixed/Improved 🛠

- Scalability Improvements
- Optimised Traffic Flow Over MQ
- Improved Peer Updates with Batching

## Known Issues 🐞

- Erratic Traffic Data In Metrics.
- Adding Custom Private/Public Key For Remote Access Gw Clients Doesn't Get Propagated To Other Peers.
- IPv6 DNS Entries Are Not Working.
- Stale Peer On The Interface, When Forced Removed From Multiple Networks At Once.
- Can Still Ping Domain Name Even When DNS Toggle Is Switched Off.
- WireGuard DNS issue on most flavours of Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Remote Access Client (RAC) and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

