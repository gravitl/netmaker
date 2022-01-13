.. Netmaker documentation master file, created by
   sphinx-quickstart on Fri May 14 08:51:40 2021.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.


.. image:: images/netmaker.png
   :width: 100%
   :alt: Netmaker WireGuard
   :align: center

.. role:: raw-html(raw)
    :format: html

:raw-html:`<br />`

=======================================
Welcome to the Netmaker Documentation
=======================================


Netmaker is a platform for creating and managing fast, secure, and dynamic virtual overlay networks using WireGuard.

This documentation covers Netmaker's :doc:`installation <./server-installation>`, :doc:`usage <./usage>`, :doc:`troubleshooting <./support>`, and customization, as well as reference documents for the :doc:`API <./api>`, UI and Agent configuration. All of the `source code <https://github.com/gravitl/netmaker>`_ for Netmaker is on GitHub.

**For Kubernetes-specific guidance, please see the** `Netmaker Kubernetes Documentation. <https://nm-k8s.readthedocs.io>`_

.. :raw-html:`<br />`

.. .. raw:: html
..   :file: youtube-1.html

About
--------

.. toctree::
   :maxdepth: 2

   about

A quick overview of Netmaker, explaining what it is, how it works, and why you should be using it.

Architecture
---------------

A technical overview of Netmaker, including design decisions and limitations.

.. toctree::
   :maxdepth: 2
   
   architecture

Install
------------------------------------

Choose the right install method for you.

.. toctree::
   :maxdepth: 1

   install

Quick Start
---------------

A quick start guide to getting up and running with Netmaker and WireGuard as quickly as possible.

.. toctree::
   :maxdepth: 2

   quick-start

.. toctree::
   :maxdepth: 2

   getting-started

Server Installation
--------------------

A detailed guide to installing the Netmaker server (API, DB, UI, DNS), and configuration options.

.. toctree::
   :maxdepth: 2
   
   server-installation

Oauth Configuration
--------------------

A simple guide to configuring OAuth for Netmaker.

.. toctree::
   :maxdepth: 2
   
   oauth


Client Installation
--------------------

A detailed guide to installing the Netmaker agent (netclient) on devices and configuration options.

.. toctree::
   :maxdepth: 2
   
   client-installation

Ingress, Egress, and Relays
------------------------------

**Ingress:** A guide on how to give machines outside of the Netmaker network access to network resources via an Ingress Gateway.

.. toctree::
   :maxdepth: 1
   
   external-clients

**Egress:** A guide on how to give machines inside the Netmaker network access to external network resources via an Egress Gateway.


.. toctree::
   :maxdepth: 1
   
   egress-gateway

**Relay:** A guide on how to make machines inside the network reachable if they are blocked by NAT/Firewall.

.. toctree::
   :maxdepth: 1
   
   relay-server


Guides
----------------

A handful of guides for use cases including site-to-site, Kubernetes, private DNS, and more.

.. toctree::
   :maxdepth: 2
   
   usage

UI Reference
---------------

A reference document for the Netmaker Server UI, with annotated screenshot detailing each field.

.. toctree::
   :maxdepth: 2

   ui-reference

API Reference
---------------

A reference document for the Netmaker Server API, and example API calls for various use cases.

**Coming Soon:** Swagger Documentation

.. toctree::
   :maxdepth: 1

   api

Upgrades
----------------

Upgrading the Netmaker server and clients.

.. toctree::
   :maxdepth: 1

   upgrades


Troubleshooting
----------------

Help with common Netmaker/netclient issues.

.. toctree::
   :maxdepth: 2

   troubleshoot


Support
----------------

Where to go for help, and a FAQ.

.. toctree::
   :maxdepth: 2

   support

Code of Conduct
-----------------

A statement on our expectations and pledge to the community.

.. toctree:: 

        conduct.rst

Licensing
---------------

A link to the Netmaker license.

.. toctree:: 

        license.rst
