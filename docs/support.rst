=========
Support
=========

FAQ
======

Does/Will Netmaker Support X Operating System?
--------------------------------------------------

Netmaker is initially available on a limited number of operating systems for good reason: Every operating system is designed differently. With a small team, we can either focus on making Netmaker do a lot on a few number of operating systems, or a little on a bunch of operating systems. We chose the first option. You can view the System Compatibility docs for more info, but in general, you should only be using Netmaker on systemd linux right now.

However, as of v0.4, we will technically be able to bring any operating system into the network. This is a bit of a hack. v0.4 introduces Ingress Gateways. Think of it this way. You set up a private network. You want devices to access it. You set up a single node as an "Ingress Gateway" and generate config files for "external clients." These clients are unmanaged and unmeshed, meaning they can access the network but only via the gateway. It also means they will not automatically account for changes to the network, and the user will have to update the configs manually.

This lets us immediately "support" any device which can run WireGuard, which includes most operating systems at this point including phones and Windows.

As we stabilize the design and feature set of Netmaker, we will expand the operating system support for Netclient which configures dynamic, fully-meshed devices. Expect to see updates about new OS support every few weeks, until eventually the Ingress Gateway becomes unnecessary (though you will still want it for certain use cases).

How do I install the Netclient on X?
---------------------------------------

As per the above, there are many unsupported operating systems. You are still welcome to try, it is just an executable binary file after all. If the system is unix-based and has kernel WireGuard installed, netclient may very well mesh the device into the network. However, the service likely will encounter problems retrieving updates.

Is Netmaker a VPN like NordNPN?
--------------------------------

No. Netmaker makes Virtual Networks, which are technically VPNs, but different. It's more like a corporate VPN, or a VPC (if you're familiar with AWS).

If you're looking to achieve self-hosted web browsing, with functionality similar to NordVPN, ExpressVPN, Surfshark, Tunnelbear, or Private Internet Access, this is probably not the project for you. Technically, you can accomplish this with Netmaker, but it would be a little like using a all-terrain vehicle for stock car racing.

There are many good projects out there that support general internet privacy using WireGuard. Here are just a few of them:

https://github.com/trailofbits/algo
https://github.com/pivpn/pivpn
https://github.com/subspacecloud/subspace
https://github.com/mullvad/mullvadvpn-app

Do you offer any paid support?
---------------------------------

Not at this time, but eventually we will. If you are interested, or if you are interested in sponsoring the project generally, please contact Alex Feiszli (alex@gravitl.com).

Why the SSPL License?
----------------------

We thought long and hard about the license. Ultimately, we think this is the best way to support and ensure the health of the project long term. The community deserves something that is well-maintained, and in order to do that, eventually we need some financial support. We won't do that by limiting the project, but we will offer some additional support, and hosted options for things people would end up paying for anyway (relay servers, load balancing support, backups). 

While SSPL is not an OSI-approved open source license, it let's people generally run the project however they want, both for private use and business use, without running into the issue of someone else monetizing the project and making it financially untenable. We are working on making the guidelines clear, and will make sure that the license does not impact the communities ability to use and modify the project.

If you have concerns about the license leading to project restrictions down the road, just know that there are other paid, closed-source/closed-core options out there, so beyond not wanting to follow that path, we also don't think it's a good idea economically either. We firmly believe that having the project open is not only right, but the best option.

All that said, we will re-evaluate the license on a regular basis and determine if an OSI-approved license makes more sense. It's just easier to move from SSPL to another license than vice-versa.

Issues, Bugs, and Feature Requests
=====================================

Issues / Bugs
----------------

Feature Requests
-------------------

Contact
===========
If you need help, try the discord or open a GitHub ticket.

Email: info@gravitl.com

Discord: https://discord.gg/zRb9Vfhk8A
