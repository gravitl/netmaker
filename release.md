# Netmaker v1.0.0

## Whats New ✨

- Multi-Factor Authentication (MFA) for user logins – added an extra layer of security to your accounts.

- Gateways Unified: Internet Gateways are now merged into the general Gateway feature and available in Community Edition.

- Improved OAuth & IDP Sync: Simplified and more reliable configuration for identity provider integrations.

- Global Map View: Visualize all your endpoints and users across the globe in a unified interface.

- Network Graph Control: Directly control and manage endpoints via the interactive network graph.

- Site-to-Site over IPv6: IPv4 site-to-site communication over IPv6 Netmaker overlay tunnels.

## 🛠 Improvements & Fixes

- Auto-Sync DNS Configs: Multi-network DNS configurations now sync automatically between server and clients.

- Stability Fixes: Improved connection reliability for nodes using Internet Gateways.

- LAN/Private Routing Enhancements: Smarter detection and handling of local/private routes, improving peer-to-peer communication in complex network environments.

## Known Issues 🐞

- WireGuard DNS issue on Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Netmaker Desktop, previously known as the Remote Access Client (RAC), and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

- Inaccurate uptime info in metrics involving ipv4-only and ipv6-only traffic

- netclients cannot auto-upgrade on ipv6-only machines.

- Need to optimize multi-network netclient join with enrollment key

