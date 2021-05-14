.. Netmaker documentation master file, created by
   sphinx-quickstart on Fri May 14 08:51:40 2021.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.


.. image:: netmaker.png
   :width: 100%
   :alt: Netmaker WireGuard
   :align: center


Welcome to the Netmaker Documentation
=======================================


Netmaker is a platform for creating and managing fast, secure, and easy-to-use virtual overlay networks using WireGuard.

This site covers Netmaker's installation, usage, troubleshooting, and customization, as well as thorough documentation of configurations for the API, UI and Agent. You can view and retrieve all of our `source code <https://github.com/gravitl/netmaker>`_ on GitHub.

.. raw:: html
   :file: youtube-1.html

About
------

A quick overview of Netmaker, explaining what it is about, and why you should be using it.

.. toctree::
   :maxdepth: 2
   
   about

Architecture
---------------

Information about Netmaker's technical design and how it is implemented.

.. toctree::
   :maxdepth: 2
   
   architecture

Quick Start
---------------

Get up and running as quickly as possible with a full mesh overlay VPN based on WireGuard.

.. toctree::
   :maxdepth: 2

   quick-start

Server Installation
--------------------

Covers installation of the Server, UI, DB, and supporting services such as Client and CoreDNS.

.. toctree::
   :maxdepth: 2
   
   server-installation

Client Installation
--------------------

Covers installation of the agent (netclient) and configuration options.

.. toctree::
   :maxdepth: 2
   
   client-installation


Using Netmaker
----------------

Different use cases such as site-to-site/gateway, Kubernetes, and DNS. Use these guides to get started with a more advanced use case.

.. toctree::
   :maxdepth: 2
   
   usage

API Reference
---------------


These are the reference documents for the Netmaker Server API. It also provides examples for various API calls that cover different use cases. The API docs are currently static. In a future release these docs will be replaced by Swagger docs.

**TODO:** Swagger Documentation via https://github.com/swaggo/swag

.. toctree::
   :maxdepth: 1

   api

Support
----------------

Common issue troubleshooting, FAQ, and Contact information.

.. toctree::
   :maxdepth: 2

   support



Code of Conduct
-----------------

Learn how to approproately interact with the community: 

.. toctree:: 

        conduct.rst

Licensing
---------------

Information about the Netmaker license.

.. toctree:: 

        license.rst
