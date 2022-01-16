=====================================
Relay Servers
=====================================

Introduction
===============

.. image:: images/relay1.png
   :width: 80%
   :alt: Relay
   :align: center

Sometimes nodes are in hard-to-reach places. Typically this will be due to a CGNAT, Double NAT, or restrictive firewall. In such scenarios, a direct peer-to-peer connection with all other nodes might be impossible.

For this reason, Netmaker has a Relay Server functionality. At any time you may designate a publicly reachable node (such as the Netmaker Server) as a Relay, and tell it which machines it should relay. Then, all traffic routing to and from that machine will go through the relay. This allows you to circumvent the above issues and ensure connectivity when direct measures do not work.

Configuring a Relay
==================================

To create a relay, you can use any node in your network, but it should have a public IP address (not behind a NAT). Your Netmaker server can be a relay server and makes for a good default choice if you are unsure of which node to select.

Simply click the relay button in the nodes list. Then, specify the nodes which it should relay. You can either enter the IP's directly, select from a list, or click "Select All."

.. image:: images/ui-7.jpg
   :width: 80%
   :alt: Relay
   :align: center

If you choose "select all" this essentially turns your network into a hub-and-spoke network. All traffic now routes over the relay node. This can create a bottleneck and slow down your network, but in some scenarios may simplify network operations.

After creation, you can change the list of relayed nodes by clicking "edit node" and editing the list (Field #12 below).

.. image:: images/ui-5.jpg
   :width: 40%
   :alt: Relay
   :align: center
