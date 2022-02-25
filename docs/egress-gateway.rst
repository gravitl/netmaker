=====================================
Egress Gateway
=====================================

Introduction
===============

.. image:: images/egress1.png
   :width: 80%
   :alt: Gateway
   :align: center

Netmaker allows your clients to reach external networks via an Egress Gateway. The Egress Gateway is a netclient which has been deployed to a server or router with access to a given subnet.

In the netmaker UI, that node is set as an "egress gateway." Range(s) are specified which this node has access to. Once created, all clients (and all new ext clients) in the network will be able to reach those ranges via the gateway.

Configuring an Egress Gateway
==================================

Configuring an Egress Gateway is very straight forward. As a prerequisite, you must know what you are trying to access remotely. For instance:

- a VPC
- a Kubernetes network
- a home network
- an office network
- a data center

After you have determined this, you must next deploy a netclient in a compatible location where the network is accessible. For instance, a Linux server or router in the office, or a Kubernetes worker node. This machine should be stable and relatively static (not expected to change its IP frequently or shut down unexpectedly).

Next, you must determine which interface to use in order to reach the internal network. As an example, lets say there is a machine in the network at 10.10.10.2, and you have deployed the netclient on a different machine. You can run 

.. code-block::

   ip route get 10.10.10.2

This should return the interface used to reach that address (e.x. "eth2")

Finally, once you have determined the interface, the subnet, and deployed your netclient, you can go to your Netmaker UI and set the node as a gateway.

.. image:: images/egress7.png
   :width: 80%
   :alt: Gateway
   :align: center

At this point simply insert the range(s) into the first field, and the interface name into the second field, and click "create".

.. image:: images/ui-6.jpg
   :width: 80%
   :alt: Gateway
   :align: center

Netmaker will set iptables rules on the node, which will then implement these rules, allowing it to route traffic from the network to the specified range(s).

Use Cases
============

1) Remote Access
-------------------

A common scenario would be to combine this with an "Ingress Gateway" to create a simple method for accessing a home or office network. Such a setup would typically have only two nodes: the ingress and egress gateways. The Ingress Gateway should usually be globally accessible, which makes the Netmaker server itself a good candidate. This means you need only the netmaker server as the Ingress, and one additional machine (in the private network you wish to reach), as the Egress.

.. image:: images/egress2.png
   :width: 80%
   :alt: Gateway
   :align: center

In some scenarios, a single node will act as both ingress and egress! For instance, you can enable acess to a VPC using your Netmaker server, deployed with a public IP. Traffic comes in over the public IP (encrypted of course) and then routes to the VPC subnet via the egress gateway.

.. image:: images/egress3.png
   :width: 50%
   :alt: Gateway
   :align: center

2) VPN / NAT Gateway
-----------------------

Most people think of a VPN as a remote server that keeps your internet traffic secure while you browse the web, or as a tool for accessing internet services in another country,using a VPN server based in that country.

These are not typical use cases for Netmaker, but can be easily enabled.

**The most important note is this: Do not use 0.0.0.0/0 as your egress gateway.** This is how you typically set up a "standard" VPN with WireGuard, however, it will not work with Netmaker. The Netclient specifically ignores gateways that overlap with local ranges (for efficiency ranges). 0.0.0.0 overlaps with everything, so it is always ignored.

Instead, use the following list of ranges:

.. code-block::

   0.0.0.0/5,8.0.0.0/7,11.0.0.0/8,12.0.0.0/6,16.0.0.0/4,32.0.0.0/3,64.0.0.0/2,128.0.0.0/3,160.0.0.0/5,168.0.0.0/6,172.0.0.0/12,172.32.0.0/11,172.64.0.0/10,172.128.0.0/9,173.0.0.0/8,174.0.0.0/7,176.0.0.0/4,192.0.0.0/9,192.128.0.0/11,192.160.0.0/13,192.169.0.0/16,192.170.0.0/15,192.172.0.0/14,192.176.0.0/12,192.192.0.0/10,193.0.0.0/8,194.0.0.0/7,196.0.0.0/6,200.0.0.0/5,208.0.0.0/4

This list encompasses the standard "public" network ranges, and ignores the standard "private" network ranges.

Simply paste this list into your "egress gateway ranges" and your clients should begin routing public-facing traffic over the gateway.

.. image:: images/egress5.png
   :width: 50%
   :alt: Gateway
   :align: center
