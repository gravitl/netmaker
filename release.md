# Netmaker v1.7.0 Release Notes 🚀

## 🚀 What’s New

### 🏢 Multi-Tenancy for MSPs (Organizations & Tenants)

Run multiple customer environments from a single Netmaker server.

- **Organizations & tenants** — Group customers under an organization; each tenant is an isolated Netmaker environment (networks, devices, users).
- **MSP license sync** — EE/MSP installs create and update orgs/tenants from the MSP license (including teardown when a tenant is removed from the license). CE and normal Pro accounts continues to use a single local default tenant.
- **Scoped access** — API and `nmctl` select the target org/tenant via `X-Organization-ID` / `X-Tenant-ID` (`--org_id` / `--tenant_id`), with `nmctl organization list` and `nmctl tenant list` for discovery.

### 🔌 TCP Proxy / WSS Uplink

Gateways can publish a **TCP/WSS uplink** so clients can reach the mesh in restrictive environments when UDP is blocked.

- Enable TCP proxy on the gateway/host (`tcp_proxy_enabled` and related listen/TLS settings).
- Clients can opt into a TCP uplink to the gateway when the proxy is enabled.
- Supports self-signed and externally terminated TLS modes for WSS endpoints.

### 🛡️ EDR Integration (Pro)

Connect endpoint detection and response platforms for **posture checks** from **Integrations**.

- Supported providers: **Microsoft Defender**, **CrowdStrike**, **SentinelOne**, and **Wazuh**.
- Sync managed endpoints and evaluate EDR compliance (agent health / risk level) as part of device posture.
- Configure, test, and manage integrations via the REST API (`/api/v1/integrations/edr/{provider}`).

### 📱 MDM Integration (Pro)

Connect mobile device management platforms for **device compliance posture** from **Integrations**.

- Supported providers: **Microsoft Intune**, **Jamf**, **JumpCloud**, and **Iru**.
- Match devices by Entra device ID, serial number, hardware UUID, or hostname.
- Enforce MDM enrollment/compliance checks alongside existing posture policies.
- Configure, test, and manage integrations via the REST API (`/api/v1/integrations/mdm/{provider}`).


---

## 🗄️ Database Schema Migration

This release completes the SQL schema path and introduces **multi-tenancy (org/tenant) bootstrap** as part of the v1.7.0 migration.

**Upgrade requirement (existing deployments):**

- You **must** run **Netmaker v1.6.0** successfully **before** upgrading to v1.7.0.
- v1.7.0 will **refuse to start** if `migration-v1.6.0` has not completed on a prior v1.6.0 deployment.
- Recommended path: deploy v1.6.0 → confirm the server starts cleanly → then upgrade to v1.7.0.


**Impact:**

- Schema and data are updated automatically on successful startup.
- Downgrades may not be supported after migration.

**👉 Action Required:**

- Do not jump from v1.5.x (or earlier) straight to v1.7.0 on an existing database.
- Ensure migrations complete and validate core functionality post-upgrade.

For detailed upgrade steps, refer to the official upgrade documentation:

[Server Upgrades v1.5.1+](https://learn.netmaker.io/getting-started/server-and-client-management/upgrading-your-client-and-server#server-upgrades-v1.5.1)

---

## 🧰 Improvements & Fixes

- **Auto-relay peer reset** — Reset a specific peer-to-peer connection that is using a relay (clear/reassign auto-relay for that peer pair) without resetting the entire network’s auto-relay state.

- **Migration reliability** — v1.7.0 blocks startup until v1.6.0 migration completed on existing deployments; migration failures exit cleanly instead of panicking.


- **Host status** — Host filtering uses live check-in status (Online/Offline/Disconnected) rather than a stale DB value.

- **MSP installs** — `nm-quick.sh` `-s` flag to skip nmctl/mesh/netclient on MSP server installs.


---

## 🐞 Known Issues

- **IPv6-only machines**  
  Netclients cannot currently **auto-upgrade** on IPv6-only systems.

- **Multi-network join performance**  
  Multi-network netclient joins using an **enrollment key** still require optimization.

- **systemd-resolved DNS limitation**  
  On systems using **systemd-resolved in uplink mode**, only the **first 3 entries** in `resolv.conf` are honored; additional entries are ignored. This may cause DNS resolution issues. **Stub mode is recommended**.

- **Windows Desktop App + mixed gateway modes**  
  When the Windows Desktop App is connected to both:
  - a **Full Tunnel Gateway**, and
  - a **Split Tunnel Gateway**

  the gateway monitoring component may disconnect from the **Split Tunnel Gateway**.
