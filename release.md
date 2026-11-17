# Netmaker v1.8.0 Release Notes 🚀

## 🚀 What’s New

### 💻 All-New Desktop Apps

Completely redesigned **Netmaker Desktop Apps** for a faster, stable connection experience.

- Now available to **CE users** as well (previously limited to Pro/EE).
- **Peer-to-peer by default** — desktop devices join the mesh with direct peering (not gateway-only), for lower latency and better connectivity.
- Fresh UI and connection flow for connecting to networks and exit nodes.
- Improved status visibility for tunnel state and exit modes.
- **Auto Exit** — when enabled on a network (`auto_select_exit_node`), desktop/user devices are automatically assigned an allowed internet exit (nearest available) when they connect; users can still switch exits when permitted.

### 🔐 Managed Access with Netmaker SSH

Secure, managed SSH access to hosts through Netmaker — without opening inbound SSH on the public internet.

- Enable **Manage SSH** on hosts so netclient runs an embedded SSH/SCP/SFTP server.
- Access mesh hosts over Netmaker with centralized control of who can connect.
- Useful for admin access, file transfer, and operational workflows on enrolled devices.

### 🗺️ Network Map Updates

Updated network map experience for clearer topology and device context.

- Improved layout and visualization of users, devices, servers, and config clients.
- Easier navigation of network relationships and host locations.
- Better at-a-glance view of how your mesh is connected.

### 🪟 Windows Egress Support

Windows hosts can now act as **egress gateways**.

- Advertise egress ranges from Windows netclients.
- Route traffic from the mesh to attached LAN/site networks via Windows.
- Uses Windows NetNat where required for egress NAT.

### 🔀 Bypass Egress Routes

New option on **internet egress** so traffic to other (site) egress destinations does not hairpin through the internet exit.

- **Why:** Without bypass, clients using an internet exit often send site-egress traffic up to the exit node and back out again (hairpin), adding latency and load. Bypass keeps allowed site egress as **direct peers**, so traffic to those sites flows peer-to-peer instead of bending through the exit.
- **`bypass_egress_routes`** (internet egress only) — enable to avoid that hairpin for other egress sites; disable to force site-egress traffic via the internet exit.
- Configurable on create/update; defaults to **on** for new internet egress.
- Ignored for non-internet (site) egress types.


---

## 🗄️ Database Schema Migration

This release runs **`migration-v1.8.0`**, which continues the SQL schema path after multi-tenancy (org/tenant) from v1.7.0. It migrates remaining key-value resources into versioned tables, backfills IP pools/allocations, and applies internet-egress defaults.

**What `migration-v1.8.0` does:**

- Migrates **extclients**, **ACLs**, **tags**, and **DNS entries** into `*_v1` schema tables (network resolved by name within tenant).
- Adds **slugs** / **UUIDs** (and role **scope**) for user groups and user roles.
- Lowercases **email usernames** where the username is an email address.
- Clears legacy **`owner_username`** values on hosts (re-established by current device ownership flows).
- Backfills **IP address pools and allocations** from existing node/extclient addresses.
- Sets **`bypass_egress_routes=true`** on existing internet egress rows that predate the option (one-shot; later admin `false` is preserved).

**Also on startup (idempotent helpers):**

- Migrates legacy **internet gateways** (`IsInternetGateway`) into **internet-type egress** resources and exit-node selections.
- Backfills **`network_global_access`** on default network-admin roles.

**Upgrade requirement (existing deployments):**

- You **must** run **Netmaker v1.7.0** successfully **before** upgrading to v1.8.0.
- v1.8.0 will **refuse to start** if `migration-v1.7.0` has not completed on a prior v1.7.0 deployment.
- Recommended path: deploy v1.7.0 → confirm the server starts cleanly → then upgrade to v1.8.0.
- **Do not** jump from v1.6.x (or earlier) straight to v1.8.0 on an existing database.

**Pre-upgrade check (IP uniqueness):**

- If a network has the **same overlay IP** on more than one node or extclient, **`migration-v1.8.0` will fail** until duplicates are resolved.
- Fix duplicate addresses in the UI/API, then retry the upgrade.

**Impact:**

- Schema and data are updated automatically on successful startup.
- Downgrades may not be supported after migration.

**👉 Action Required:**

- Confirm `migration-v1.7.0` completed on v1.7.0 before moving to v1.8.0.
- Resolve any duplicate overlay IPs before upgrading.
- Ensure migrations complete and validate networks, ACLs, tags, DNS, and exit/egress behavior post-upgrade.

For detailed upgrade steps, refer to the official upgrade documentation:

[Server Upgrades](https://learn.netmaker.io/getting-started/server-and-client-management/upgrading-your-client-and-server)

---

## 🧰 Improvements & Fixes

- **Migration reliability** — v1.8.0 blocks startup until `migration-v1.7.0` completed on existing deployments; IP pool backfill fails clearly on duplicate addresses instead of corrupting allocations.

- **IP address management** — Networks gain managed IP pools/allocations; pools are synced on startup so existing networks can allocate addresses safely.

- **Auto-relay performance** — Lower CPU use for auto-relay; API to list auto-relayed peers for a node; reset a specific auto-relayed peer pair without resetting the whole network.

- **User-device / desktop ACL paths** — CE supports user ACL policies with All Users and per-user sources; AllowAll is preserved when both default policies are open; FwUpdate calc skipped for user-owned devices where appropriate.

- **Exit node + relay** — Selecting an exit node can replace an existing relay; alternate exit peers limited to user devices (servers continue to hairpin via the selected exit).

- **Host / device auth** — Reject host-password JWTs for user-owned devices; AuthorizeHost accepts a user session plus `X-Host-ID`. Network-role API access tightened for network admins.

- **Extclient reliability** — Bulk create API and faster creation; mark extclients offline when their ingress gateway is unreachable; keep static WireGuard configs across JIT, IdP, and access cleanup paths.

- **Topology / map data** — Topology and connections enriched with location, gateway flags, and egress context.

- **Device join resilience** — Device join/pull tolerates `host.Nodes` drift vs live nodes.

- **Account lifecycle** — Force logout and user-device cleanup when disabling/deleting user accounts; username filter on nodes API.

- **Email usernames** — Email-style usernames normalized to lowercase (invites and migration).

- **Traffic keys** — AES key derivation via ECDH; stronger validation of traffic public keys.


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

- **Duplicate overlay IPs block v1.8.0 migration**  
  Networks with the same address on multiple nodes/extclients must be cleaned up before `migration-v1.8.0` can complete.
