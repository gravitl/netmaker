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


.. :raw-html:`<br />`

.. .. raw:: html
..   :file: youtube-1.html

About
------
A quick overview of Netmaker, explaining what it is, how it works, and why you should be using it.

.. toctree::
   :maxdepth: 2
   
   about

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

Quick Start Nginx (depreciated)
------------------------------------

An older guide to getting up and running with Netmaker using Nginx as quickly as possible.

.. toctree::
   :maxdepth: 1

   quick-start-nginx

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

External Clients
--------------------

A detailed guide to give clients outside of the Netmaker network access to network resources.

.. toctree::
   :maxdepth: 2
   
   external-clients

Guides
----------------

A handful of guides for use cases including site-to-site, Kubernetes, private DNS, and more.

.. toctree::
   :maxdepth: 2
   
   usage

API Reference
---------------

A reference document for the Netmaker Server API, and example API calls for various use cases.

**Coming Soon:** Swagger Documentation

.. toctree::
   :maxdepth: 1

   api

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
