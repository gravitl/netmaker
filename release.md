# Netmaker v0.24.2

## Whats New ✨
- Improved Funtionality of static host with separate settings for port and endpoint ip
- Network Info and Metadata info added to Remote-Access-Client

## What's Fixed/Improved 🛠
- Improved FailOver Functionality
- Local Peer Routing in Dual-Stack Environment
- Stale Node Issue On Multinet When Deleting Host
- IPv6 Internet Gateways Improvements
- Handled New Oauth User SignUp via Remote-Access-Client
- PeerUpdate Improvements around default host and multi-nets

## Known Issues 🐞

- Erratic Traffic Data In Metrics
- `netclient server leave` Leaves a Stale Node Record In At Least One Network When Part Of Multiple Networks, But Can Be Deleted From The UI.
- IPv6 internet traffic does not route to the InetGw in Dual Stack Network
