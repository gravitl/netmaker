===============
About
===============

Confused about what Netmaker is? That's ok, you're in the right place. Networking is hard, and you're not alone, but Netmaker is here to make your network nightmare into a dreamy breeze.

Introduction
===============

Netmaker is a tool for creating and managing virtual overlay networks. If you have servers spread across multiple locations, data centers, or clouds, this platform will make life easier. 

Netmaker takes all those machines and puts them into a single, secure, flat network so that they can all talk to each other easily and securely. It's like a VPC but of arbitrary computers. From the machine's perspective, they're in the same room with their buddy machines, even if they're spread all over the world.

Netmaker is similar to Tailscale, ZeroTier, or Nebula. What makes Netmaker different is its speed and flexibility. NNetmaker is faster because it uses kernel WireGuard. It's more dynamic because the server and agents are fully configurable, and let you do all sorts on things to meet special use cases.

How Does Netmaker Work?
=======================

Netmaker is two things: The admin panel/server, and the netclient. You interact with the admin panel, creating networks and managing machines/access. The server just holds onto configurations that will be loaded by the machines. The machines use the netclient to retrieve the configs and set WireGuard, a special encrypted tunneling tool. These configs tell each machine how to reach each of the other machines.

Traffic is only routed through the main server if you want it to. The server can crash and everything will run fine until machine configs change (you just wont be able to add/remove/update machines from the network).

Use Cases
=========
 1. Create a flat, secure network between multiple/hybrid cloud environments
 2. Integrate central and edge services
 3. Provide access to IoT devices, remote locations, or client sites.
 3. Secure a home or office network while providing remote connectivity
 4. Manage cryptocurrency proof-of-stake machines
 6. Provide an additional layer of security on an existing network
 7. Encrypt Kubernetes inter-node communications
 8. Secure site-to-site connections
