# Netmaker v0.24.3

## Whats New ✨
- Validation Checks For Egress Routes
- Network Change Detection System
- Removed Creation Of ACLs For EMQX

## What's Fixed/Improved 🛠
- Removed RAG Metadata Length Restriction
- Scalability Improvements
- Optimised Traffic Flow Over MQ
- Improved Validation Checks For Internet GWS

## Known Issues 🐞

- Erratic Traffic Data In Metrics.
- Adding Custom Private/Public Key For Remote Access Gw Clients Doesn't Get Propagated To Other Peers.
- IPv6 DNS Entries Are Not Working.
- Stale Peer On The Interface, When Forced Removed From Multiple Networks At Once.
- Can Still Ping Domain Name Even When DNS Toggle Is Switched Off.
- WireGuard DNS issue on most flavors of Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Remote Access Client (RAC) and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

