# Netmaker v0.99.0

## Whats New ✨

IDP Integration: Seamless integration with Google Workspace and Microsoft Entra ID, including automatic synchronization of users and groups

User Activity & Audit Logs: Comprehensive tracking of control plane events such as user management, node changes, ACL modifications, and user access events.

Updated Egress UI: A redesigned interface for managing egress gateways for improved usability.

User Access API Tokens: Generate and manage API tokens for user-level access and automation.

Server Settings via Dashboard: View and configure core server settings directly from the web dashboard.

ACLs on Community Edition (Beta): The new version of Access Control Lists is now available in CE as a beta feature.

New Metrics Page: Gain better insights with a revamped metrics dashboard.

Offline Node Auto-Cleanup: Automatically remove stale or inactive nodes to keep networks clean.

## 🛠 Improvements & Fixes

Optimized DNS Query Handling: Faster and more efficient internal name resolution.

Improved Failover Handling: Enhanced stability and signaling for backup peer connections.

User Egress Policies: More granular control over user-level outbound traffic policies.

LAN/Private Routing Enhancements: Better detection and handling of local/private endpoint routes during peer communication.

## Known Issues 🐞

- WireGuard DNS issue on Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Netmaker Desktop previously Remote Access Client (RAC) and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

