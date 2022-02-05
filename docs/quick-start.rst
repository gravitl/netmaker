===============
Quick Install
===============

This quick start guide is an **opinionated** guide for getting up and running with Netmaker as quickly as possible.

If just trialing netmaker, you may also want to check out the 3-minute PoC install of Netmaker in the README on GitHub. The following is just a guided version of that script, plus a custom domain (instead of nip.io): https://github.com/gravitl/netmaker .

Introduction
==================

We assume for this installation that you want all of the Netmaker features enabled, you want your server to be secure, and you want your server to be accessible from anywhere.

This instance will not be HA. However, it should comfortably handle around one hundred concurrent clients and support the most common use cases.

If you are deploying for a business or enterprise use case and this setup will not fit your needs, please contact info@gravitl.com, or check out the business subscription plans at https://gravitl.com/plans/business.

By the end of this guide, you will have Netmaker installed on a public VM linked to your custom domain, secured behind a Caddy reverse proxy.

For information about deploying more advanced configurations, see the :doc:`Advanced Installation <./server-installation>` docs. 


0. Prerequisites
==================
-  **Virtual Machine**
   
   - Preferably from a cloud provider (e.x: DigitalOcean, Linode, AWS, GCP, etc.)
   
   - (We do not recommend Oracle Cloud, as VM's here have been known to cause network interference.)

   - Public, static IP 
   
   - Min 1GB RAM, 1 CPU (4GB RAM, 2CPU preferred for production installs)
   
   - 2GB+ of storage 
   
   - Ubuntu  20.04 Installed

- **Domain**

  - A publicly owned domain (e.x. example.com, mysite.biz) 
  - Permission and access to modify DNS records via DNS service (e.x: Route53)

1. Prepare DNS
================

Create a wildcard A record pointing to the public IP of your VM. As an example, *.netmaker.example.com.

Caddy will create 3 subdomains with this wildcard, EX:

- dashboard.netmaker.example.com

- api.netmaker.example.com

- grpc.netmaker.example.com


2. Install Dependencies
========================

.. code-block::

  ssh root@your-host
  sudo apt-get update
  sudo apt-get install -y docker.io docker-compose wireguard

At this point you should have all the system dependencies you need.
 
3. Open Firewall
===============================

Make sure firewall settings are set for Netmaker both on the VM and with your cloud security groups (AWS, GCP, etc). 

Make sure the following ports are open both on the VM and in the cloud security groups:

- **443 (tcp):** for Dashboard, REST API, and gRPC
- **53 (udp and tcp):** for CoreDNS
- **51821-518XX (udp):** for WireGuard - Netmaker needs one port per network, starting with 51821, so open up a range depending on the number of networks you plan on having. For instance, 51821-51830.

.. code-block::

  sudo ufw allow proto tcp from any to any port 443 && sudo ufw allow 53/udp && sudo ufw allow 53/tcp && sudo ufw allow 51821:51830/udp

**Again, based on your cloud provider, you may additionally need to set inbound security rules for your server (for instance, on AWS). This will be dependent on your cloud provider. Be sure to check before moving on:**
  - allow 443/tcp from all
  - allow 53/udp and 53/tcp from all
  - allow 51821-51830/udp from all


4. Install Netmaker
========================

Prepare Docker Compose 
------------------------

**Note on COREDNS_IP:** Depending on your cloud provider, the public IP may not be bound directly to the VM on which you are running. In such cases, CoreDNS cannot bind to this IP, and you should use the IP of the default interface on your machine in place of COREDNS_IP. This command will get you the correct IP for CoreDNS in many cases:

.. code-block::

  ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p'

Now, insert the values for your base (wildcard) domain, public ip, and coredns ip.

.. code-block::

  wget -O docker-compose.yml https://raw.githubusercontent.com/gravitl/netmaker/master/compose/docker-compose.contained.yml
  sed -i 's/NETMAKER_BASE_DOMAIN/<your base domain>/g' docker-compose.yml
  sed -i 's/SERVER_PUBLIC_IP/<your server ip>/g' docker-compose.yml
  sed -i 's/COREDNS_IP/<default interface ip>/g' docker-compose.yml

Generate a unique master key and insert it:

.. code-block::

  tr -dc A-Za-z0-9 </dev/urandom | head -c 30 ; echo ''
  sed -i 's/REPLACE_MASTER_KEY/<your generated key>/g' docker-compose.yml

You may want to save this key for future use with the API.

Prepare Caddy
------------------------

.. code-block::

  wget -O /root/Caddyfile https://raw.githubusercontent.com/gravitl/netmaker/master/docker/Caddyfile

  sed -i 's/NETMAKER_BASE_DOMAIN/<your base domain>/g' /root/Caddyfile
  sed -i 's/YOUR_EMAIL/<your email>/g' /root/Caddyfile

Start Netmaker
----------------

``sudo docker-compose up -d``

navigate to dashboard.<your base domain> to begin using Netmaker.

To troubleshoot issues, start with:

``docker logs netmaker``

Or check out the :doc:`troubleshoooting docs <./troubleshoot>`.
