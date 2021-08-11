=========
Support
=========

FAQ
======

Does/Will Netmaker Support X Operating System?
--------------------------------------------------

Netmaker is initially available on a limited number of operating systems for good reason: Every operating system is designed differently. With a small team, we can either focus on making Netmaker do a lot on a few number of operating systems, or a little on a bunch of operating systems. We chose the first option. You can view the System Compatibility docs for more info, but in general, you should only be using Netmaker on systemd linux right now.

However, via "external clients", any device that supports WireGuard can be added to the network. 

In future iterations will expand the operating system support for Netclient, and devices that must use the "external client" feature can switch to Netclient.

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

Do you offer any enterprise support?
--------------------------------------

If you are interested in enterprise support for your project, please contact info@gravitl.com.


Why the SSPL License?
----------------------

We thought long and hard about the license. Ultimately, we think this is the best way to support and ensure the health of the project long term. The community deserves something that is well-maintained, and in order to do that, eventually we need some financial support. We won't do that by limiting the project, but we will offer some additional support, and hosted options for things people would end up paying for anyway (relay servers, load balancing support, backups). 

While SSPL is not an OSI-approved open source license, it let's people generally run the project however they want, both for private use and business use, without running into the issue of someone else monetizing the project and making it financially untenable. We are working on making the guidelines clear, and will make sure that the license does not impact the communities ability to use and modify the project.

If you have concerns about the license leading to project restrictions down the road, just know that there are other paid, closed-source/closed-core options out there, so beyond not wanting to follow that path, we also don't think it's a good idea economically either. We firmly believe that having the project open is not only right, but the best option.

All that said, we will re-evaluate the license on a regular basis and determine if an OSI-approved license makes more sense. It's just easier to move from SSPL to another license than vice-versa.


Contact
===========
If you need help, try the discord or open a GitHub ticket.

Email: info@gravitl.com

Discord: https://discord.gg/zRb9Vfhk8A
