=================
Getting Started
=================

Once you have Netmaker installed via the :doc:`Quick Install <./quick-start>` guide, you can use this Getting Started guide to help create and manage your first network.

Setup
=================

#. Create your admin user, with a username and password.
#. Login with your new user
#. Create your first network by clicking on Create Network

Create a Network
=================


.. image:: images/create-net.png
   :width: 80%
   :alt: Create Network Screen
   :align: center

This network should have a sensible name (nodes will use it to set their interfaces).

More importantly, it should have a non-overlapping, private address range. 

If you are running a small (less than 254 machines) network, and are unsure of which CIDR's to use, you could consider:

- 10.11.12.0/24
- 10.20.30.0/24
- 100.99.98.0/24

Network Settings Description
-------------------------------

The Network creation form has a few fields which may seem unfamiliar. Here is a brief description:

**UDP Hole Punching:** UDP Hole Punching enables the server to perform STUN. This means, when nodes check in, the server will record return addresses and ports. It will then communicate this information to the other nodes when they check in, allowing them to reach their peers more easily. This has two benefits. For one, it%. It also means, you dont usually have to worry about opening up the local firewall for ports (for instance, 51821). **This setting is usually good to turn on, with some noteable exceptions.** This setting cannot be enabled if "client mode" is turned off. This setting can also break peer-to-peer functionality if, for whatever reason, nodes are unable to reach the server.

**Is Local Network:**  This is almost always best to leave this turned off and is left for very special circumstances. If you are running a data center or a private WAN, you may want to enable this setting. It defines the range that nodes will set for Endpoints. Usually, Endpoints are just the public IP. But in some cases, you don't want any nodes to be reachable via a public IP, and instead want to use a private range.

**Is Dual Stack:** This setting adds ipv6 private addresses to nodes, in addition to ipv4 addresses. Usually, this is unnecessary, but in some cases, you may have a requirement for ipv6 and can enable this setting.

Once your network is created, you should see that the netmaker server has added itself to the network. From here, you can move on to adding additional nodes to the network.

.. image:: images/netmaker-node.png
   :width: 80%
   :alt: Node Screen
   :align: center


Create a Key
===============

Adding nodes to the network typically requires a key.

#. Click on the ACCESS KEYS tab and select the network you created.
#. Click ADD NEW ACCESS KEY
#. Give it a name (ex: "mykey") and a number of uses (ex: 25)
#. Click CREATE KEY (**Important:** Do not click out of the following screen until you have saved your key details. It will appear only once.)
#. Copy the bottom command under "Your agent install command with access token" and save it somewhere locally. E.x: ``curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/netclient-install.sh | KEY=vm3ow4thatogiwnsla3thsl3894ths sh -``.

.. image:: images/access-key.png
   :width: 80%
   :alt: Access Key Screen
   :align: center

You will use this command to install the netclient on your nodes. There are three different values for three different scenarios: 

* The **Access Key** value is the secret string that will allow your node to authenticate with the Netmaker network. This can be used with existing netclient installations where additional configurations (such as setting the server IP manually) may be required. This is not typical. E.g. ``netclient join -k <access key> -s grpc.myserver.com -p 50051``
* The **Access Token** value is a base64 encoded string that contains the server IP and grpc port, as well as the access key. This is decoded by the netclient and can be used with existing netclient installations like this: ``netclient join -t <access token>``. You should use this method for adding a network to a node that is already on a network. For instance, Node A is in the **mynet** network and now you are adding it to **default**.
* The **install command** value is a curl command that can be run on Linux systems. It is a simple script that downloads the netclient binary and runs the install command all in one.
  
Networks can also be enabled to allow nodes to sign up without keys at all. In this scenario, nodes enter a "pending state" and are not permitted to join the network until an admin approves them.

Deploy Nodes
=================

0. Prereqisite: Every machine on which you install should have wireguard and systemd already installed.

1. SSH to each machine 
2. ``sudo su -``
3. **Prerequisite Check:** Every Linux machine on which you run the netclient must have WireGuard and systemd installed
4. For linux machines with SystemD and WireGuard installed, Run the install command, Ex: ``curl -sfL https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/netclient-install.sh | KEY=vm3ow4thatogiwnsla3thsl3894ths sh -``
5. For Mac, Windows, and arch-specific linux distributions (e.g. ARM), `download the appropriate netclient for your system <https://github.com/gravitl/netmaker/releases/tag/latest/>`_ . Then, run "netclient join -t <your token>".

You should get output similar to the below. The netclient retrieves local settings, submits them to the server for processing, and retrieves updated settings. Then it sets the local network configuration. For more information about this process, see the :doc:`client installation <./client-installation>` documentation. If this process failed and you do not see your node in the console (see below), then reference the :doc:`troubleshooting <./troubleshoot>` documentation.

.. image:: images/nc-install-output.png
   :width: 80%
   :alt: Output from Netclient Install
   :align: center


.. image:: images/nm-node-success.png
   :width: 80%
   :alt: Node Success
   :align: center


Repeat the above steps for every machine you would like to add to your network. You can re-use the same install command so long as you do not run out of uses on your access key (after which it will be invalidated and deleted).

Once installed on all nodes, you can test the connection by pinging the private address of any node from any other node.


.. image:: images/ping-node.png
   :width: 80%
   :alt: Node Success
   :align: center

Manage Nodes
===============

Your machines should now be visible in the control pane. 

.. image:: images/nodes.png
   :width: 80%
   :alt: Node Success
   :align: center

You can view/modify/delete any node by selecting it in the NODES tab. For instance, you can change the name to something more sensible like "workstation" or "api server". You can also modify network settings here, such as keys or the WireGuard port. These settings will be picked up by the node on its next check in. For more information, see Advanced Configuration in the :doc:`Using Netmaker <./usage>` docs.

.. image:: images/node-details.png
   :width: 80%
   :alt: Node Success
   :align: center



Nodes can be added/removed/modified on the network at any time. Nodes can also be added to multiple Netmaker networks. Any changes will get picked up by any nodes on a given network, and will take aboue ~30 seconds to take effect.

Uninstalling the netclient
=============================

1. To remove your nodes from the default network, run the following on each node: ``sudo netclient leave -n default``
2. To remove the netclient entirely from each node, run ``sudo rm -rf /etc/netclient`` (after running the first step)

Uninstalling Netmaker
===========================

To uninstall Netmaker from the server, simply run ``docker-compose down`` or ``docker-compose down --volumes`` to remove the docker volumes for a future installation.

