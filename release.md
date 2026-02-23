## Netmaker v1.5.0 Release Notes 🚀 

## 🚀 What’s New

### 🔓 Just-In-Time Access (beta)

- Time-limited, on-demand network access: users request access, admins approve or deny, and grants expire automatically.

- Request/approval workflow with configurable grant duration; admins retain full control over who accesses which networks and when.

### 🔁 Overlapping Egress Ranges (beta)

- Virtual NAT mode enables multiple egress routers to share overlapping IP ranges by assigning each egress a virtual range from a configurable pool.
- Configurable per-network IPv4 pool and site prefix length for virtual range allocation.
- Eliminates routing conflicts when multiple sites need to egress the same destination CIDRs (e.g., multiple offices routing to the same cloud VPC).
- Supports both direct NAT and virtual NAT modes for flexible egress configurations.

### 🌍 Gateway Monitoring

- Desktop App connections automatically fail over to healthy gateway hubs when the primary becomes unavailable.
- Gateway health is monitored via connectivity checks and last-seen metrics; only online gateways are used for new connections.

## 🧰 Improvements & Fixes

- **IP Detection Interval** User can now choose the Device Endpoint IP detection interval based on their requirements.

- **User Migration:** Optimized user migration logic to reduce server startup time.

- **DNS:** Use Global Nameservers only if no match-all nameservers are configured, added fallback nameserver configuration.

- **Darwin:** Netclients on macOS can now use internet gateway.

- **GeoLocation:** Consolidate IP location API usage with fallbacks


## Known Issues 🐞

- netclients cannot auto-upgrade on ipv6-only machines.

- Need to optimize multi-network netclient join with enrollment key

- On systems using systemd-resolved in uplink mode, the first 3 entries in resolv.conf are used and rest are ignored. So it might cause DNS issues. Stub mode is preferred.

- When a Windows desktop app is connected to a Full Tunnel Gateway, and a Split Tunnel Gateway at the same time,
    the gateway monitoring component would disconnect from the split tunnel gateway.
