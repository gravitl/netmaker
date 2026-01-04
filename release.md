## Netmaker v1.4.0 Release Notes 🚀 

## 🚀 What’s New

### 🌍 Posture Checks (beta)

- Security feature that validates device compliance against configured policies based on device attributes such as OS, OS version, kernel version, client version, geographic location, and auto-update status.
- Supports tag-based and user group-based assignment of posture checks to specific devices or users.
- Tracks violations with configurable severity levels and provides real-time evaluation of device compliance.
- Helps ensure only compliant devices can access network resources.

### 🔁 Network Traffic Logging (alpha)

- Comprehensive network flow logging system that captures and stores network traffic metadata in ClickHouse.
- Tracks source and destination IPs, ports, protocols, bytes/packets sent/received, and connection timestamps.
- Provides API endpoints for querying flow data with filters by network, node, user, protocol, and time range.
- Enables network administrators to monitor, analyze, and audit network traffic patterns for security and troubleshooting purposes.

### 🌐 K8s Operator with Cluster Access, Egress and Ingress functionality (beta)

- **Cluster Egress**: Expose Netmaker network services to Kubernetes workloads using standard Service names.
- **Cluster Ingress**: Expose Kubernetes services to devices on your Netmaker network.
- **API Proxy**: Secure access to Kubernetes API servers through Netmaker tunnels with RBAC support.


### 🔄 Auto Removal of Offline Peers

- Automatically removes nodes that have been offline for a configurable threshold period.
- Configurable per network with customizable timeout thresholds (in minutes).
- Supports tag-based filtering to selectively apply auto-removal to specific device groups.
- Helps maintain clean network topology by removing stale or abandoned peer connections.

### 🧩 Onboarding Flow

- Streamlined user onboarding experience during signup for workspace setup.


## 🧰 Improvements & Fixes

- Azure IDP sync: Fixed User sync by group filters.

- User Migration: Optimised User migration logic to reduce server start up time.

- Config Files: Avoid Auto enabling of configs on user login.

- Egress Domain Updates: Fixed domain-related issues in egress configurations to ensure consistent routing behavior.

## Known Issues 🐞

- netclients cannot auto-upgrade on ipv6-only machines.

- Need to optimize multi-network netclient join with enrollment key

- On systems using systemd-resolved in uplink mode, the first 3 entries in resolv.conf are used and rest are ignored. So it might cause DNS issues. Stub mode is preferred.
