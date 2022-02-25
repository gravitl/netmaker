=========
Support
=========

FAQ
======

Is Netmaker a VPN like NordNPN?
--------------------------------

No. Netmaker makes Virtual Networks, which are technically VPNs, but different. It's more like a corporate VPN, or a VPC (if you're familiar with AWS). Netmaker is often compared to OpenVPN, Tailscale, or Nebula.

If you're looking to achieve self-hosted web browsing, with functionality similar to NordVPN, ExpressVPN, Surfshark, Tunnelbear, or Private Internet Access, this is probably not the project for you. Technically, you can accomplish this with Netmaker, but it would be a little like using a all-terrain vehicle for stock car racing.

There are many good projects out there that support general internet privacy using WireGuard. Here are just a few of them:

https://github.com/trailofbits/algo
https://github.com/pivpn/pivpn
https://github.com/subspacecloud/subspace
https://github.com/mullvad/mullvadvpn-app

Do you have an 'Exit Nodes' feature?
---------------------------------------

Please see the :doc:`Egress Gateway <./egress-gateway>` documentation.

Do you offer any business or enterprise support?
---------------------------------------------------

Yes, please contact info@gravitl.com or visit https://gravitl.com/plans.


Why the SSPL License?
----------------------

As of now, we think the SSPL is the best way to ensure the long-term viability of the project, but we are regularly evaluating this to see if an OSI-approved license makes more sense.

We believe the SSPL lets most people run the project the way they want, for both for private use and business use, while giving us a path to maintain viability. We are working to make sure the guidelines clear, and do not want the license to impact the community's ability to use and modify the project.

If you believe the SSPL will negatively impact your ability to use the project, please do not hesitate to reach out.

Telemetry
==============

As of v0.10.0, Netmaker collects "opt-out" telemetry data. To opt out, simply set "TELEMETRY=off" in your docker-compose file.

Please consider participating in telemetry, as it helps us focus on the features and bug fixes which are most useful to users. Netmaker is a broad platform, and without this data, it is difficult to know where the team should spend its limited resources.

The following is the full list of telemetry data we collect. Besides "Server Version" all data is simply an integer count:

- Randomized server ID
- Count of nodes
- Count of "non-server" nodes
- Count of external clients
- Count of networks
- Count of users
- Count of linux nodes
- Count of freebsd nodes
- Count of macos nodes
- Count of windows nodes
- Count of docker nodes
- Count of k8s nodes
- Server version

We use  `PostHog <https://https://posthog.com/>`_, an open source and trusted framework for telemetry data.

To look at exactly we collect telemetry, you can view the source code under serverctl/telemetry.go: https://github.com/gravitl/netmaker/blob/master/serverctl/telemetry.go

Contact
===========
If you need help, try the discord or open a GitHub ticket.

Email: info@gravitl.com

Discord: https://discord.gg/zRb9Vfhk8A
