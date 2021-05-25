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

The netclient is intended to be run as a system daemon. This allows it to automatically retrieve and send updates. To do this, the netclient can install itself as a systemd service.

This requires a systemd-based linux operating system.

If running the netclient on a non-systemd system, it is recommended to manually configure the netclient as a daemon using whatever method is acceptable on the chosen operating system.

Private DNS Management
-----------------------

To manage private DNS, the netclient relies on systemd-resolved (resolvectl). Absent this, it cannot set private DNS for the machine.

A user may choose to manually set a private DNS nameserver of <netmaker server>:53. However, beware, as netmaker sets split dns, and the system must be configured properly. Otherwise, this nameserver may break your local DNS.

Prerequisites
=============

**For netclient cli:** Linux/Unix with WireGuard installed (wg command available)

**For netclient daemon:** Systemd Linux + WireGuard

**For Private DNS management:** Resolvectl (systemd-resolved)

Configuration
===============

Variable Reference
--------------------

Config File Reference
------------------------

CLI Reference
------------------------

Installation
======================

Token
-------

Access Key
------------

Manual
---------

Config File
------------

Managing Netclient
=====================

Viewing Logs
---------------

Making Updates
----------------

Adding/Removing Networks
---------------------------

Uninstalling
---------------

Troubleshooting
-----------------

