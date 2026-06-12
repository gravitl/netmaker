# Netmaker v1.6.0 Release Notes 🚀

## 🚀 What’s New

### 🔁 Site-to-Site ACLs (Beta)

Define ACL policies that permit traffic between egress endpoints across networks.

- Build site-to-site rules between egress resources on different networks.
- Combine egress resources, nodes, and specific IPs in a single policy.
- Site-to-site rules are emitted alongside device-mesh rules without key collisions.


### 🛡️ Egress ACLs with IP Restriction

ACL policies can now target **individual IPs** inside an egress range using the `ip` ACL target type.

- Restrict access to specific hosts within a larger egress CIDR.
- Validate that selected IPs fall within the referenced egress range at policy create/update time.
- Mix egress resources, nodes, tags, and individual IPs in the same policy.

### 📦 Egress Preset Catalog (Pro)

A built-in catalog simplifies domain-based egress for common SaaS and cloud providers.

- Browse presets via `GET /api/v1/egress/presets` (AWS, Azure, Google, Salesforce, and more).
- Create egress resources from a `preset_id`; the server can resolve AWS IP ranges automatically.
- Support for **multiple domains** per egress resource.

### ⏱️ JIT Group Memberships

Just-In-Time (JIT) access can now be scoped to **user groups** per network.

- Enable JIT for all non-admin users, or limit it to selected user groups.
- Users request access; admins approve or deny with email notifications.
- Expired grants are cleaned up automatically and users are notified.

### 🔗 SIEM Integration

Forward Netmaker audit events to your security stack from **Integrations**.

- Supported providers: **Splunk**, **Datadog**, **Elastic**, and **Microsoft Sentinel**.
- Configure, test, and manage integrations via the REST API (`/api/v1/integrations/siem/{provider}`).
- Events are exported through the SIEM exporter service.

### 🔑 Default Enrollment Keys

Networks can designate a **default enrollment key** for simplified device onboarding.

- Fetch the default key per network via the API or CLI.
- Regenerate enrollment key tokens without recreating the key.

---

## 🗄️ Database Schema Migration

This release introduces schema changes to the following core entities:

- Nodes
- Pending Users
- User Invites
- Posture Check Violations

**Impact:**

- The database structure will be updated automatically during the upgrade.
- Downgrades may not be supported after migration.

**👉 Action Required:**

- Ensure the application starts successfully and migrations are complete.
- Validate core functionality post-upgrade.

For detailed upgrade steps, refer to the official upgrade documentation:

[Server Upgrades v1.5.1+](https://learn.netmaker.io/getting-started/server-and-client-management/upgrading-your-client-and-server#server-upgrades-v1.5.1)

---

## 🧰 Improvements & Fixes

- **Netclient registration UX** — Host registration over OAuth/basic auth now returns clear websocket close reasons on failure (auth errors, missing access, posture violations, and server errors).

- **User group management** — Streamlined user role permissions and group updates, role-downgrade handling.

- **Orphan reference cleanup** — Removes stale network references left behind after resource deletion.

- **Scalability & reliability** — Optimized node status calculation, offline-status hooks, zombie/orphan node cleanup, and ACL cache race fixes.

- **API hardening** — Auth rate limiting on REST endpoints and activity-log permission fixes.

- **Egress improvements** — CIDR validation for ACL egress IPs, multi-domain egress routing, and domain-answer handling for preset-based egress.

- **Failover removed** — Legacy per-node failover APIs and CLI commands have been removed in favor of gateway-based patterns.

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
