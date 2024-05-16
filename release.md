# Netmaker v0.24.2

## Whats New ✨
- Users Can define Multiple Endpoints On The Remote Access Gateway To Choose From While Establishing a Connection.
- OAUTH Code Moved From CE To Pro.
- nm-quick.sh Enhancement To Install The Latest Docker To Enable Support Of the Latest Distros.
- IPv6 Enhancements.

## What's Fixed/Improved 🛠

- Egress Enhancement In Multiple Networks
- Fix armv5-v7 Upgrade Download Link
- Fix Windows Interface Issue In Multiple Networks
- SSO network join Improvements.
- Remove Egress Routes After Egress Gateway Removed
- Remote Access Gateway Connection Handling Improvements.

## Known Issues 🐞

- Erratic Traffic Data In Metrics
- `netclient server leave` Leaves a Stale Node Record In At Least One Network When Part Of Multiple Networks, But Can Be Deleted From The UI.
- IPv6 internet traffic does not route to the InetGw in Dual Stack Network
