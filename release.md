# Netmaker v0.30.0

## Whats New ✨
- Advanced ACL Rules - port, protocol and traffic direction
- Reduced Firewall Requirements To One Single Port (443 udp/tcp)
- Option to Turn off STUN or specify custom stun servers
- Improved Connectivity Status Indicator with real-time troubleshooting help.
- Optimised MQ message size

## What's Fixed/Improved 🛠
- Metrics Data
- FailOver Stability Fixes
- Scalability Fixes
- Duplicate Node IP check on update

## Known Issues 🐞

- Adding Custom Private/Public Key For Remote Access Gw Clients Doesn't Get Propagated To Other Peers.
- IPv6 DNS Entries Are Not Working.
- Stale Peer On The Interface, When Forced Removed From Multiple Networks At Once.
- WireGuard DNS issue on most flavours of Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Remote Access Client (RAC) and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

