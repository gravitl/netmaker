====================
Client Installation
====================

This document tells you how to install the netclient on machines that will be a part of your Netmaker network, as well as non-compatible systems.

These steps should be run after the Netmaker server has been created and a network has been designated within Netmaker.

Introduction to Netclient
===============================

At its heart, the netclient is a simple CLI for managing access to various WireGuard-based networks. It manages WireGuard on the host system, so that you don't have to. Why is this necessary?

If you are setting up a WireGuard-based virtual network, you must configure each machine with very specific settings, so that every machine can reach it, and it can reach every machine. Any changes to the settings of any one of these machines can break those connections. Any machine that is added, removed, or modified on the network requires reconfiguring every peer in the network. This can be very time consuming.

The netmaker server holds configuration details about every machine in your network and how other machines should connect to it.

The netclient agent connects to the server, pushing and pulling information when the network (or its local configuration) changes. 

The netclient agent then configures WireGuard (and other network properties) locally, so that the network stays intact.

Notes on Windows
==================================

If running the netclient on windows, you must download the netclient.exe binary and run it from Powershell as an Administrator.

Windows will by default have firewall rules that prevent inbound connections. If you wish to allow inbound connections from particular peers, use the following command:

``netsh advfirewall firewall add rule name="Allow from <peer private addr>" dir=in action=allow protocol=ANY remoteip=<peer private addr>``

If you want to allow all peers access, but do not want to configure firewall rules for all peers, you can configure access for one peer, and set it as a Relay Server.

Modes and System Compatibility
==================================

**Note: If you would like to connect non-Linux/Unix machines to your network such as phones and Windows desktops, please see the documentation on External Clients**

The netclient can be run in a few "modes". System compatibility depends on which modes you intend to use. These modes can be mixed and matched across a network, meaning all machines do not have to run with the same "mode."

CLI
------------

In its simplest form, the netclient can be treated as just a simple, manual, CLI tool, which a user can call to configure the machine. The cli can be compiled from source code to run on most systems, and has already been compiled for x86 and ARM devices.

As a CLI, the netclient should function on any Linux or Unix based system that has the wireguard utility (callable with **wg**) installed.

Daemon
----------

The netclient is intended to be run as a system daemon. This allows it to automatically retrieve and send updates. To do this, the netclient can install itself as a systemd service, or launchd/windows service for Mac or Windows.

If running the netclient on non-systemd linux, it is recommended to manually configure the netclient as a daemon using whatever method is acceptable on the chosen operating system.

Private DNS Management
-----------------------

To manage private DNS, the netclient relies on systemd-resolved (resolvectl). Absent this, it cannot set private DNS for the machine.

A user may choose to manually set a private DNS nameserver of <netmaker server>:53. However, beware, as netmaker sets split dns, and the system must be configured properly. Otherwise, this nameserver may break your local DNS.

Prerequisites
=============

To obtain the netclient, go to the GitHub releases: https://github.com/gravitl/netmaker/releases

**For netclient cli:** Linux/Unix with WireGuard installed (wg command available)

**For netclient daemon:** Systemd Linux + WireGuard

**For Private DNS management:** Resolvectl (systemd-resolved)

Configuration
===============

The CLI has information about all commands and variables. This section shows the "help" output for these commands as well as some additional reference.

CLI Reference
--------------------
``sudo netclient --help``

.. literalinclude:: ./examplecode/netclient-help.txt
  :language: YAML


``sudo netclient join --help``

.. literalinclude:: ./examplecode/netclient-join.txt
  :language: YAML


Config File Reference
------------------------

There is a config file for each node under /etc/netconfig-<network name>. You can change these values and then set "postchanges" to "true", or go to the CLI and run ``netclient push -n <network>``


.. literalinclude:: ./examplecode/netconfig-example.yml
  :language: YAML


Installation
======================


To install netmaker, you need a server token for a particular network, unless you're joining a network that allows manual signup, in which case you can join without a token, but the server will quarantine the machine until the admin approves it.

An admin creates a token in the ACCESS KEYS section of the UI. Upon creating a token, it generates 3 values:

**Access Key:** The secret key to authenticate as a node in the network

**Access Token:** The secret key plus information about how to access the server (addresses, ports), all decoded by the netclient to register with the server

**Install Command:** A short script that will obtain the netclient binary, register with the server, and join the network, all in one

For first time installations, you can run the Install Command. For additional networks, simply run ``netclient join -t <access token>``. The raw access key will not be needed unless there are special circumstances, mostly troubleshooting incorrect information in the token (you can instead manually specify the server location).


Managing Netclient
=====================

Viewing Logs
---------------

**to view current networks**
  ``netclient list``

**to tail logs**
  ``journalctl -u netclient@<net name> -f``

**to view all logs**
  ``journalctl -u netclient@<net name>``

**to get most recent log run**
  ``systemctl status netclient@<net name>``

Making Updates
----------------

``vim /etc/netclient/netconfig-<network>``

Change any of the variables in this file, and changes will be pushed to the server and processed locally on the next checkin.

For instance, change the private address, endpoint, or name. See above example config file for details


Adding/Removing Networks
---------------------------

``netclient join -t <token>``

Set any of the above flags (netclient join --help) to override settings for joining the network. 
If a key is provided (-k), then a token is unnecessary, but grpc, server, ports, and network must all be provided via flags.


Uninstalling
---------------

``netclient uninstall``


