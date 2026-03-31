# Netmaker v1.5.1 Release Notes 🚀

## 🚀 What’s New

### 🔁 Traffic Logs (Beta)

Traffic Logs have now moved into **Beta**.

- Traffic Logs are now enriched with relevant **domain tagging**, making network activity easier to audit and investigate.

---

## 🧰 Improvements & Fixes

- **Scalability & Reliability Improvements**
  Introduced a peer update debouncer that coalesces rapid-fire PublishPeerUpdate calls into a single broadcast — a 500ms resettable debounce window capped by a 3s max-wait deadline ensures back-to-back operations (bulk node updates, gateway changes, host deletions) produce one peer update instead of dozens, drastically reducing CPU and MQTT pressure on the control plane

  Pre-warms peer update caches after each debounced broadcast so pull requests from hosts are served instantly from cache instead of triggering expensive on-demand computation

  Batched metrics export to netmaker exporter via periodic ticker instead of publishing on every individual MQTT metrics message, reducing continuous CPU pressure from Prometheus scraping

- **Database Schema Migration**  
  Added schema migrations for the **Users, Groups, Roles, Networks, and Hosts** tables.

- **Deprecated Legacy Acls**
  Legacy Acls have been completed removed now from the platform.
  
- **Paginated APIs**  
  Introduced pagination support for **Users** and **Hosts** APIs.

- **DNS**  
  Added **native Active Directory support**.

- **Posture Checks**  
  Nodes can now **skip the auto-update check during join**, improving join reliability in controlled environments.

- **IDP Sync**  
  Improved identity provider sync behavior:
  - Synced IDP groups are now **denied access by default** until explicitly granted.
  - **Okta-specific settings** are now reset when an IDP integration is removed.

- **HA Setup**  
  Streamlined **high availability (HA)** setup and operational workflows.

- **Install Script**  
  Added **on-demand Monitoring Stack installation** support via:  
  `./nm-quick.sh -m`

- **Monitoring Stack**  
  Updated the monitoring stack to use the **official Prometheus and Grafana images**.

- **HA Gateways**
  Reset Auto Assigned gw when it is disconnected from the network.

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