## Netmaker v1.1.0 Release Notes 🚀 

## What’s New ✨ 

- Okta IDP Integration – Seamless authentication and user provisioning with Okta.

- Egress Domain-Based Routing – Route traffic based on domain names, not just network CIDRs.

- DNS Nameservers with Match Domain Functionality – Fine-grained DNS resolution control per domain.

- Service User Management – Platform Network Admins can now add service users directly to networks.

- Device Approval Workflow – Require admin approval before devices can join a network.

- Auto-Created User Group Policies – Automatically generate network access policies for new user groups.

- User Session Expiry Controls – Set session timeouts for both Dashboard and Client Apps.

## Improvements & Fixes 🛠 

- Access Control Lists (ACLs): Enhanced functionality and flexibility.

- User Management UX: Streamlined workflows for easier administration.

- IDP User/Group Filtering: Improved filtering capabilities for large organizations.

- Stability Enhancements: More reliable connections for nodes using Internet Gateways.

## Known Issues 🐞

- WireGuard DNS issue on Ubuntu 24.04 and some other newer Linux distributions. The issue is affecting the Netmaker Desktop, previously known as the Remote Access Client (RAC), and the plain WireGuard external clients. Workaround can be found here https://help.netmaker.io/en/articles/9612016-extclient-rac-dns-issue-on-ubuntu-24-04.

- Inaccurate uptime info in metrics involving ipv4-only and ipv6-only traffic

- netclients cannot auto-upgrade on ipv6-only machines.

- Need to optimize multi-network netclient join with enrollment key

